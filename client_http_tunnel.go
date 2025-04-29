package gortsplib

import (
	"bufio"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
	"golang.org/x/sync/errgroup"
)

// clientHTTPTunnel implements a bidirectional RTSP-over-HTTP tunnel.
// It follows the Apple's tunneling protocol which uses two TCP connections:
// - One for reading (HTTP GET)
// - One for writing (HTTP POST)
type clientHTTPTunnel struct {
	client        *Client
	baseURL       string
	readConn      net.Conn
	writeConn     net.Conn
	sessionCookie string
	mutex         sync.Mutex
	
	// Read buffering
	readBuffer    []byte
	encBuffer     []byte
	decBuffer     []byte
	
	// For handling incomplete base64 blocks
	partialData   []byte
}

// newClientHTTPTunnel creates a new HTTP tunnel.
func newClientHTTPTunnel(client *Client, scheme, host string) (*clientHTTPTunnel, error) {
	baseURL := fmt.Sprintf("%s://%s/", scheme, host)
	
	// Generate a random session cookie
	cookie := make([]byte, 16)
	if _, err := rand.Read(cookie); err != nil {
		return nil, liberrors.ErrClientHTTPTunnelSetupFailed{Err: err}
	}
	
	sessionCookie := base64.StdEncoding.EncodeToString(cookie)
	
	return &clientHTTPTunnel{
		client:        client,
		baseURL:       baseURL,
		sessionCookie: sessionCookie,
		readBuffer:    make([]byte, httpTunnelDefaultBufferSize),
		encBuffer:     make([]byte, base64.StdEncoding.EncodedLen(httpTunnelDefaultBufferSize)),
		decBuffer:     make([]byte, httpTunnelDefaultBufferSize),
		partialData:   make([]byte, 0, 3), // Max 3 bytes can be held between reads for base64 alignment
	}, nil
}

// connect establishes both HTTP tunnel connections (GET and POST).
func (t *clientHTTPTunnel) connect() error {
	// Use errgroup to manage concurrent connection establishment
	group := new(errgroup.Group)
	
	// Establish GET connection
	group.Go(t.connectRead)
	
	// Establish POST connection
	group.Go(t.connectWrite)
	
	// Wait for both connections to be established or return the first error
	return group.Wait()
}

// createTLSConn establishes a TCP/TLS connection based on the URL scheme
func (t *clientHTTPTunnel) createTLSConn() (net.Conn, error) {
	// Establish TCP connection
	dialCtx, dialCtxCancel := t.client.ctx, t.client.ctxCancel
	conn, err := t.client.DialContext(dialCtx, "tcp", canonicalAddr(t.client.connURL))
	if err != nil {
		dialCtxCancel()
		return nil, liberrors.ErrClientHTTPTunnelConnectionFailed{Err: err}
	}
	
	// Apply TLS if needed (HTTPS)
	if t.client.connURL.Scheme == "https" {
		tlsConfig := t.client.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}
		tlsConfig.ServerName = t.client.connURL.Hostname()

		// Convert the connection to TLS
		tlsConn := tls.Client(conn, tlsConfig)
		
		// Perform handshake
		if err = tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, liberrors.ErrClientHTTPTunnelConnectionFailed{Err: err}
		}
		
		conn = tlsConn
	}
	
	return conn, nil
}

// createHTTPRequest creates an HTTP request for the tunnel endpoint
func (t *clientHTTPTunnel) createHTTPRequest(method, suffix string, includeTransferEncoding bool) string {
	url := t.baseURL + suffix
	reqStr := fmt.Sprintf("%s %s HTTP/1.1\r\n", method, url)
	reqStr += fmt.Sprintf("Host: %s\r\n", t.client.connURL.Host)
	reqStr += fmt.Sprintf("User-Agent: %s\r\n", t.client.UserAgent)
	reqStr += fmt.Sprintf("Content-Type: %s\r\n", httpTunnelContentType)
	reqStr += fmt.Sprintf("Cookie: %s=%s\r\n", httpTunnelCookieName, t.sessionCookie)
	reqStr += "Connection: Keep-Alive\r\n"
	
	if includeTransferEncoding {
		reqStr += "Transfer-Encoding: chunked\r\n"
	}
	
	reqStr += "\r\n"
	return reqStr
}

// connectRead establishes the HTTP GET connection for reading data.
func (t *clientHTTPTunnel) connectRead() error {
	// Create TCP/TLS connection
	conn, err := t.createTLSConn()
	if err != nil {
		return err
	}
	
	// Set read timeout
	conn.SetReadDeadline(time.Now().Add(t.client.ReadTimeout))
	
	// Send HTTP GET request
	reqStr := t.createHTTPRequest("GET", httpTunnelGetSuffix, false)
	if _, err = conn.Write([]byte(reqStr)); err != nil {
		conn.Close()
		return liberrors.ErrClientHTTPTunnelSetupFailed{Err: err}
	}
	
	// Read HTTP response
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		conn.Close()
		return liberrors.ErrClientHTTPTunnelSetupFailed{Err: err}
	}
	
	// Check response status
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return liberrors.ErrClientHTTPTunnelRequestFailed{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
		}
	}
	
	// Store the connection
	t.readConn = conn
	return nil
}

// connectWrite establishes the HTTP POST connection for writing data.
func (t *clientHTTPTunnel) connectWrite() error {
	// Create TCP/TLS connection
	conn, err := t.createTLSConn()
	if err != nil {
		return err
	}
	
	// Set write timeout
	conn.SetWriteDeadline(time.Now().Add(t.client.WriteTimeout))
	
	// Send HTTP POST request
	reqStr := t.createHTTPRequest("POST", httpTunnelPostSuffix, true)
	if _, err = conn.Write([]byte(reqStr)); err != nil {
		conn.Close()
		return liberrors.ErrClientHTTPTunnelSetupFailed{Err: err}
	}
	
	// Store the connection
	t.writeConn = conn
	return nil
}

// Read reads data from the tunnel (HTTP GET response body).
// It handles base64 decoding of received data in blocks of 4 bytes.
func (t *clientHTTPTunnel) Read(b []byte) (int, error) {
	if t.readConn == nil {
		return 0, liberrors.ErrClientHTTPTunnelConnectionFailed{Err: fmt.Errorf("read connection not established")}
	}
	
	// Set read deadline
	t.readConn.SetReadDeadline(time.Now().Add(t.client.ReadTimeout))
	
	// Calculate maximum encoded length for buffer
	maxEncLen := base64.StdEncoding.EncodedLen(len(b))
	if maxEncLen > len(t.encBuffer) {
		maxEncLen = len(t.encBuffer)
	}
	
	// Prepare encoding data buffer, starting with any partial data
	encData := make([]byte, 0, maxEncLen)
	if len(t.partialData) > 0 {
		encData = append(encData, t.partialData...)
		t.partialData = t.partialData[:0] // Clear partial data
	}
	
	// Read more encoded data
	n, err := t.readConn.Read(t.encBuffer[:maxEncLen-len(encData)])
	if err != nil {
		return 0, err
	}
	encData = append(encData, t.encBuffer[:n]...)
	
	// Handle base64 alignment (must be multiple of 4)
	remainder := len(encData) % 4
	if remainder > 0 {
		dataToProcess := encData[:len(encData)-remainder]
		// Save remainder for next read
		t.partialData = append(t.partialData, encData[len(encData)-remainder:]...)
		encData = dataToProcess
	}
	
	// Nothing to decode
	if len(encData) == 0 {
		return 0, nil
	}
	
	// Decode base64 data
	decodedN, err := base64.StdEncoding.Decode(b, encData)
	if err != nil && strings.Contains(err.Error(), "illegal base64 data") {
		// Try to recover from base64 errors by processing valid blocks
		validLen := (len(encData) / 4) * 4
		if validLen > 0 {
			decodedN, err = base64.StdEncoding.Decode(b, encData[:validLen])
			if err != nil {
				return 0, err
			}
		} else {
			// Save for next read if we can't process
			t.partialData = append(t.partialData, encData...)
			return 0, nil
		}
	} else if err != nil {
		return 0, err
	}
	
	return decodedN, nil
}

// Write writes data to the tunnel (HTTP POST request body).
// It handles base64 encoding and sending data as HTTP chunks.
func (t *clientHTTPTunnel) Write(b []byte) (int, error) {
	if t.writeConn == nil {
		return 0, liberrors.ErrClientHTTPTunnelConnectionFailed{Err: fmt.Errorf("write connection not established")}
	}
	
	t.mutex.Lock()
	defer t.mutex.Unlock()
	
	// Set write deadline
	t.writeConn.SetWriteDeadline(time.Now().Add(t.client.WriteTimeout))
	
	// Encode data to base64
	encodedLen := base64.StdEncoding.EncodedLen(len(b))
	encodedData := t.encBuffer
	if encodedLen > len(t.encBuffer) {
		encodedData = make([]byte, encodedLen)
	} else {
		encodedData = encodedData[:encodedLen]
	}
	
	base64.StdEncoding.Encode(encodedData, b)
	
	// Create and write HTTP chunk header, data, and footer in one operation if possible
	chunk := fmt.Sprintf("%x\r\n", len(encodedData))
	
	// Try to combine chunk header, data, and footer in one write operation
	// to reduce syscalls when data is small enough
	if len(chunk) + len(encodedData) + 2 <= len(t.readBuffer) {
		// We can fit everything in our buffer
		combined := t.readBuffer[:0]
		combined = append(combined, chunk...)
		combined = append(combined, encodedData...)
		combined = append(combined, "\r\n"...)
		
		_, err := t.writeConn.Write(combined)
		if err != nil {
			return 0, err
		}
	} else {
		// Write in separate operations for larger data
		if _, err := t.writeConn.Write([]byte(chunk)); err != nil {
			return 0, err
		}
		
		if _, err := t.writeConn.Write(encodedData); err != nil {
			return 0, err
		}
		
		if _, err := t.writeConn.Write([]byte("\r\n")); err != nil {
			return 0, err
		}
	}
	
	return len(b), nil
}

// Close closes both HTTP tunnel connections.
func (t *clientHTTPTunnel) Close() error {
	// Use errgroup to close both connections
	group := new(errgroup.Group)
	
	if t.readConn != nil {
		conn := t.readConn
		t.readConn = nil
		group.Go(func() error {
			return conn.Close()
		})
	}
	
	if t.writeConn != nil {
		conn := t.writeConn
		t.writeConn = nil
		group.Go(func() error {
			// Write the final chunk to end the request
			conn.Write([]byte("0\r\n\r\n")) //nolint:errcheck
			return conn.Close()
		})
	}
	
	return group.Wait()
}

// LocalAddr returns the local network address.
func (t *clientHTTPTunnel) LocalAddr() net.Addr {
	if t.readConn != nil {
		return t.readConn.LocalAddr()
	}
	return nil
}

// RemoteAddr returns the remote network address.
func (t *clientHTTPTunnel) RemoteAddr() net.Addr {
	if t.readConn != nil {
		return t.readConn.RemoteAddr()
	}
	return nil
}

// SetDeadline sets the read and write deadlines.
func (t *clientHTTPTunnel) SetDeadline(tm time.Time) error {
	group := new(errgroup.Group)
	
	if t.readConn != nil {
		conn := t.readConn
		group.Go(func() error {
			return conn.SetDeadline(tm)
		})
	}
	
	if t.writeConn != nil {
		conn := t.writeConn
		group.Go(func() error {
			return conn.SetDeadline(tm)
		})
	}
	
	return group.Wait()
}

// SetReadDeadline sets the read deadline.
func (t *clientHTTPTunnel) SetReadDeadline(tm time.Time) error {
	if t.readConn != nil {
		return t.readConn.SetReadDeadline(tm)
	}
	return nil
}

// SetWriteDeadline sets the write deadline.
func (t *clientHTTPTunnel) SetWriteDeadline(tm time.Time) error {
	if t.writeConn != nil {
		return t.writeConn.SetWriteDeadline(tm)
	}
	return nil
}