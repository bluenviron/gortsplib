package gortsplib

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bluenviron/gortsplib/v5/pkg/auth"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
)

type clientTunnelHTTP struct {
	readChan  net.Conn
	readBuf   *bufio.Reader
	writeChan net.Conn
}

func (c *clientTunnelHTTP) Read(p []byte) (n int, err error) {
	return c.readBuf.Read(p)
}

func (c *clientTunnelHTTP) Write(p []byte) (n int, err error) {
	return c.writeChan.Write([]byte(base64.StdEncoding.EncodeToString(p)))
}

func (c *clientTunnelHTTP) Close() error {
	c.readChan.Close()
	c.writeChan.Close()
	return nil
}

func (c *clientTunnelHTTP) LocalAddr() net.Addr {
	return c.readChan.LocalAddr()
}

func (c *clientTunnelHTTP) RemoteAddr() net.Addr {
	return c.readChan.RemoteAddr()
}

func (c *clientTunnelHTTP) SetDeadline(_ time.Time) error {
	panic("unimplemented")
}

func (c *clientTunnelHTTP) SetReadDeadline(t time.Time) error {
	return c.readChan.SetReadDeadline(t)
}

func (c *clientTunnelHTTP) SetWriteDeadline(t time.Time) error {
	return c.writeChan.SetWriteDeadline(t)
}

func newClientTunnelHTTP(
	ctx context.Context,
	addr string,
	secure bool,
	tlsConfig *tls.Config,
	dialContext func(ctx context.Context, network, address string) (net.Conn, error),
	dialTLSContext func(ctx context.Context, network string, addr string) (net.Conn, error),
	u *base.URL,
) (net.Conn, error) {
	c := &clientTunnelHTTP{}

	if secure {
		// clone TLS config and fill ServerName if empty.
		// this is the same behavior of http.Client.
		// https://cs.opensource.google/go/go/+/master:src/net/http/transport.go;l=1754;drc=a4b534f5e42fe58d58c0ff0562d76680cedb0466

		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		} else {
			tlsConfig = tlsConfig.Clone()
		}

		if tlsConfig.ServerName == "" {
			host, _, _ := net.SplitHostPort(addr)
			tlsConfig.ServerName = host
		}
	}

	dial := func() (net.Conn, error) {
		if secure && dialTLSContext != nil {
			return dialTLSContext(ctx, "tcp", addr)
		}

		nconn, err := dialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}

		if secure {
			nconn = tls.Client(nconn, tlsConfig)
		}

		return nconn, nil
	}

	tunnelID := strings.ReplaceAll(uuid.New().String(), "-", "")
	requestTarget := clientTunnelHTTPRequestTarget(u)

	// the tunnel requests are authenticated like RTSP requests:
	// credentials are sent only after the server asked for them, with the
	// method it asked for. The POST request, whose response is never read,
	// reuses the method negotiated by the GET request.
	var sender *auth.Sender

	authorization := func(method string) string {
		if sender == nil {
			return ""
		}
		return "Authorization: " + sender.Authorization(method, requestTarget)[0] + "\r\n"
	}

	for {
		readChan, err := dial()
		if err != nil {
			return nil, err
		}

		var statusCode int
		var wwwAuth base.HeaderValue
		readBuf := bufio.NewReader(readChan)

		err = runWithContext(ctx, readChan, func() error {
			// do not use http.Request
			// since Content-Length requires a Body of same size
			_, err2 := readChan.Write([]byte(
				"GET " + requestTarget + " HTTP/1.1\r\n" +
					"Host: " + addr + "\r\n" +
					"X-Sessioncookie: " + tunnelID + "\r\n" +
					"Accept: application/x-rtsp-tunnelled\r\n" +
					authorization("GET") +
					"\r\n",
			))
			if err2 != nil {
				return err2
			}

			res, err2 := http.ReadResponse(readBuf, nil)
			if err2 != nil {
				return err2
			}
			res.Body.Close()

			statusCode = res.StatusCode
			wwwAuth = base.HeaderValue(res.Header.Values("WWW-Authenticate"))
			return nil
		})
		if err != nil {
			readChan.Close()
			return nil, err
		}

		if statusCode == http.StatusUnauthorized && sender == nil && u != nil && u.User != nil {
			// send the request again with authentication, on a new connection,
			// since the server may close the one that carried the challenge.
			readChan.Close()

			pass, _ := u.User.Password()

			sender = &auth.Sender{
				WWWAuth: wwwAuth,
				User:    u.User.Username(),
				Pass:    pass,
			}
			err = sender.Initialize()
			if err != nil {
				return nil, liberrors.ErrClientAuthSetup{Err: err}
			}
			continue
		}

		if statusCode != http.StatusOK {
			readChan.Close()
			return nil, fmt.Errorf("bad status code: %v", statusCode)
		}

		c.readChan = readChan
		c.readBuf = readBuf
		break
	}

	ok := false

	defer func() {
		if !ok {
			c.readChan.Close()
		}
	}()

	var err error
	c.writeChan, err = dial()
	if err != nil {
		return nil, err
	}

	err = runWithContext(ctx, c.writeChan, func() error {
		// do not use http.Request
		// since Content-Length requires a Body of same size
		_, err2 := c.writeChan.Write([]byte(
			"POST " + requestTarget + " HTTP/1.1\r\n" +
				"Host: " + addr + "\r\n" +
				"X-Sessioncookie: " + tunnelID + "\r\n" +
				"Content-Type: application/x-rtsp-tunnelled\r\n" +
				"Content-Length: 30000\r\n" +
				authorization("POST") +
				"\r\n",
		))
		return err2
	})
	if err != nil {
		c.writeChan.Close()
		return nil, err
	}

	// do not wait for writeChan response, since some servers don't send it.

	ok = true
	return c, nil
}

// runWithContext runs fn and closes nconn if ctx is done before fn returns,
// so that a blocked read or write returns.
func runWithContext(ctx context.Context, nconn net.Conn, fn func() error) error {
	done := make(chan struct{})
	terminate := make(chan struct{})

	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
			nconn.Close()
		case <-terminate:
		}
	}()

	err := fn()

	close(terminate)
	<-done

	return err
}

func clientTunnelHTTPRequestTarget(u *base.URL) string {
	if u == nil {
		return "/"
	}

	ret := u.Path
	if ret == "" {
		ret = "/"
	}

	if u.RawQuery != "" {
		ret += "?" + u.RawQuery
	}

	return ret
}
