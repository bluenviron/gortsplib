package gortsplib

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/stretchr/testify/require"
)

type mockHTTPServer struct {
	getConn     net.Conn
	postConn    net.Conn
	sessionCookie string
	server     *http.Server
	listener   net.Listener
	t          *testing.T
}

func newMockHTTPServer(t *testing.T) (*mockHTTPServer, string) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &mockHTTPServer{
		t:        t,
		listener: listener,
	}

	s.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract cookie value
			cookieStr := r.Header.Get("Cookie")
			require.Contains(t, cookieStr, httpTunnelCookieName)
			
			// Process the actual cookie value
			cookies := r.Cookies()
			var cookieVal string
			for _, c := range cookies {
				if c.Name == httpTunnelCookieName {
					cookieVal = c.Value
					break
				}
			}
			require.NotEmpty(t, cookieVal)
			
			if s.sessionCookie == "" {
				s.sessionCookie = cookieVal
			} else {
				require.Equal(t, s.sessionCookie, cookieVal)
			}

			require.Equal(t, httpTunnelContentType, r.Header.Get("Content-Type"))

			hijacker, ok := w.(http.Hijacker)
			require.True(t, ok)

			conn, _, err := hijacker.Hijack()
			require.NoError(t, err)

			if strings.Contains(r.URL.String(), "protocol=get") {
				s.getConn = conn
				conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			} else if strings.Contains(r.URL.String(), "protocol=post") {
				s.postConn = conn
				conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
			} else {
				conn.Close()
				require.Fail(t, "Unknown URL path")
			}
		}),
	}

	go s.server.Serve(listener)

	addr := fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
	return s, addr
}

func (s *mockHTTPServer) close() {
	if s.getConn != nil {
		s.getConn.Close()
	}
	if s.postConn != nil {
		s.postConn.Close()
	}
	s.server.Close()
	s.listener.Close()
}

// Simulates sending data from server to client
func (s *mockHTTPServer) simulateServerToClientData(data []byte) {
	require.NotNil(s.t, s.getConn)
	
	// Encode data to base64
	encodedData := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(encodedData, data)
	
	_, err := s.getConn.Write(encodedData)
	require.NoError(s.t, err)
}

// Reads data coming from client to server
func (s *mockHTTPServer) readClientToServerData(readTimeout time.Duration) []byte {
	require.NotNil(s.t, s.postConn)
	
	s.postConn.SetReadDeadline(time.Now().Add(readTimeout))
	
	// Read a chunk
	buf := make([]byte, 1024)
	n, err := s.postConn.Read(buf)
	require.NoError(s.t, err)
	
	// Parse the chunk header
	data := string(buf[:n])
	parts := strings.Split(data, "\r\n")
	require.GreaterOrEqual(s.t, len(parts), 3)
	
	// Parse chunk size (ignoring the result, just verifying format)
	var size int
	_, err = fmt.Sscanf(parts[0], "%x", &size)
	require.NoError(s.t, err)
	
	// Extract the base64 data
	encodedData := []byte(parts[1])
	
	// Decode base64 data
	decodedData := make([]byte, base64.StdEncoding.DecodedLen(len(encodedData)))
	n, err = base64.StdEncoding.Decode(decodedData, encodedData)
	require.NoError(s.t, err)
	
	return decodedData[:n]
}

func TestClientHTTPTunnel(t *testing.T) {
	server, serverAddr := newMockHTTPServer(t)
	defer server.close()

	u, err := base.ParseURL(serverAddr)
	require.NoError(t, err)

	// Create client and HTTP tunnel
	client := &Client{
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{}).DialContext,
	}
	client.ctx, client.ctxCancel = context.WithCancel(context.Background())
	client.connURL = u

	tunnel, err := newClientHTTPTunnel(client, u.Scheme, u.Host)
	require.NoError(t, err)
	
	// Connect
	err = tunnel.connect()
	require.NoError(t, err)
	defer tunnel.Close()
	
	// Test bidirectional data transfer
	testData := []byte("RTSP test data 1234")
	
	// Write data to server
	n, err := tunnel.Write(testData)
	require.NoError(t, err)
	require.Equal(t, len(testData), n)
	
	// Server should receive data
	serverReceivedData := server.readClientToServerData(1 * time.Second)
	require.Equal(t, testData, serverReceivedData)
	
	// Server sends data to client
	server.simulateServerToClientData(testData)
	
	// Client should receive data
	buf := make([]byte, 1024)
	n, err = tunnel.Read(buf)
	require.NoError(t, err)
	require.Equal(t, testData, buf[:n])
}

func TestClientHTTPTunnelPartialRead(t *testing.T) {
	server, serverAddr := newMockHTTPServer(t)
	defer server.close()

	u, err := base.ParseURL(serverAddr)
	require.NoError(t, err)

	// Create client and HTTP tunnel
	client := &Client{
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{}).DialContext,
	}
	client.ctx, client.ctxCancel = context.WithCancel(context.Background())
	client.connURL = u

	tunnel, err := newClientHTTPTunnel(client, u.Scheme, u.Host)
	require.NoError(t, err)
	
	// Connect
	err = tunnel.connect()
	require.NoError(t, err)
	defer tunnel.Close()
	
	// Test handling of partial base64 encoded data
	// We'll send 5 bytes which encodes to 8 bytes in base64
	// Then send 7 more bytes for a total of 12 bytes which encode to 16 bytes
	testData1 := []byte("12345")
	testData2 := []byte("1234567")
	
	// First test data: 5 bytes
	server.simulateServerToClientData(testData1)
	
	// Client should receive first data chunk
	buf := make([]byte, 1024)
	n, err := tunnel.Read(buf)
	require.NoError(t, err)
	require.Equal(t, testData1, buf[:n])
	
	// Second test data: 7 bytes
	server.simulateServerToClientData(testData2)
	
	// Client should receive second data chunk
	n, err = tunnel.Read(buf)
	require.NoError(t, err)
	require.Equal(t, testData2, buf[:n])
}

func TestClientHTTPTunnelErrors(t *testing.T) {
	server, serverAddr := newMockHTTPServer(t)
	defer server.close()

	u, err := base.ParseURL(serverAddr)
	require.NoError(t, err)

	// Create client and HTTP tunnel
	client := &Client{
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
		DialContext: (&net.Dialer{}).DialContext,
	}
	client.ctx, client.ctxCancel = context.WithCancel(context.Background())
	client.connURL = u

	tunnel, err := newClientHTTPTunnel(client, u.Scheme, u.Host)
	require.NoError(t, err)
	
	// Before connecting, operations should fail
	_, err = tunnel.Read(make([]byte, 10))
	require.Error(t, err)
	
	_, err = tunnel.Write([]byte("test"))
	require.Error(t, err)
	
	// Connect
	err = tunnel.connect()
	require.NoError(t, err)
	
	// Close
	err = tunnel.Close()
	require.NoError(t, err)
	
	// After closing, operations should fail
	_, err = tunnel.Read(make([]byte, 10))
	require.Error(t, err)
	
	_, err = tunnel.Write([]byte("test"))
	require.Error(t, err)
}

// Most of the actual TLS behavior is tested in integration tests,
// but here we at least verify the scheme is handled correctly
func TestClientHTTPTunnelTLSScheme(t *testing.T) {
	client := &Client{
		ReadTimeout: 5 * time.Second,
		WriteTimeout: 5 * time.Second,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true, // For testing only
		},
	}
	
	// Use https scheme
	u, err := base.ParseURL("https://example.com")
	require.NoError(t, err)
	client.connURL = u

	tunnel, err := newClientHTTPTunnel(client, u.Scheme, u.Host)
	require.NoError(t, err)
	
	// We're not actually going to connect, just verify the scheme behavior
	require.Equal(t, "https://example.com/", tunnel.baseURL)

	// HTTP scheme
	u, err = base.ParseURL("http://example.com")
	require.NoError(t, err)
	client.connURL = u

	tunnel, err = newClientHTTPTunnel(client, u.Scheme, u.Host)
	require.NoError(t, err)
	
	require.Equal(t, "http://example.com/", tunnel.baseURL)
}