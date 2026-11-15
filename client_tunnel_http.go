package gortsplib

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
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
	onHTTPRequest ClientOnHTTPRequestFunc,
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

	if secure && dialTLSContext != nil {
		var err error
		c.readChan, err = dialTLSContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		c.readChan, err = dialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}

		if secure {
			c.readChan = tls.Client(c.readChan, tlsConfig)
		}
	}

	ok := false

	defer func() {
		if !ok {
			c.readChan.Close()
		}
	}()

	ctxCheckerReadDone := make(chan struct{})
	defer func() { <-ctxCheckerReadDone }()

	ctxCheckerReadTerminate := make(chan struct{})
	defer close(ctxCheckerReadTerminate)

	go func() {
		defer close(ctxCheckerReadDone)
		select {
		case <-ctx.Done():
			c.readChan.Close()
		case <-ctxCheckerReadTerminate:
		}
	}()

	tunnelID := strings.ReplaceAll(uuid.New().String(), "-", "")
	requestTarget := clientTunnelHTTPRequestTarget(u)

	err := writeClientTunnelHTTPRequest(c.readChan, onHTTPRequest, &http.Request{
		Method: http.MethodGet,
		URL:    &url.URL{Opaque: requestTarget},
		Host:   addr,
		Header: http.Header{
			"X-Sessioncookie": []string{tunnelID},
			"Accept":          []string{"application/x-rtsp-tunnelled"},
		},
	})
	if err != nil {
		return nil, err
	}

	c.readBuf = bufio.NewReader(c.readChan)
	res, err := http.ReadResponse(c.readBuf, nil)
	if err != nil {
		return nil, err
	}
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %v", res.StatusCode)
	}

	if secure && dialTLSContext != nil {
		c.writeChan, err = dialTLSContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
	} else {
		c.writeChan, err = dialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}

		if secure {
			c.writeChan = tls.Client(c.writeChan, tlsConfig)
		}
	}

	defer func() {
		if !ok {
			c.writeChan.Close()
		}
	}()

	ctxCheckerWriteDone := make(chan struct{})
	defer func() { <-ctxCheckerWriteDone }()

	ctxCheckerWriteTerminate := make(chan struct{})
	defer close(ctxCheckerWriteTerminate)

	go func() {
		defer close(ctxCheckerWriteDone)
		select {
		case <-ctx.Done():
			c.writeChan.Close()
		case <-ctxCheckerWriteTerminate:
		}
	}()

	err = writeClientTunnelHTTPRequest(c.writeChan, onHTTPRequest, &http.Request{
		Method: http.MethodPost,
		URL:    &url.URL{Opaque: requestTarget},
		Host:   addr,
		Header: http.Header{
			"X-Sessioncookie": []string{tunnelID},
			"Content-Type":    []string{"application/x-rtsp-tunnelled"},
			"Content-Length":  []string{"30000"},
		},
	})
	if err != nil {
		return nil, err
	}

	// do not wait for writeChan response, since some servers don't send it.

	ok = true
	return c, nil
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

// writeClientTunnelHTTPRequest writes the request line and headers only,
// since http.Request.Write requires a Body as long as Content-Length.
func writeClientTunnelHTTPRequest(
	w io.Writer,
	onHTTPRequest ClientOnHTTPRequestFunc,
	req *http.Request,
) error {
	onHTTPRequest(req)

	var buf bytes.Buffer
	buf.WriteString(req.Method + " " + req.URL.RequestURI() + " HTTP/1.1\r\n")
	buf.WriteString("Host: " + req.Host + "\r\n")
	req.Header.Write(&buf) //nolint:errcheck
	buf.WriteString("\r\n")

	_, err := w.Write(buf.Bytes())
	return err
}
