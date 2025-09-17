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
	dialContext func(ctx context.Context, network, address string) (net.Conn, error),
	addr string,
	tlsConfig *tls.Config,
) (net.Conn, error) {
	c := &clientTunnelHTTP{}

	var err error
	c.readChan, err = dialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		c.readChan = tls.Client(c.readChan, tlsConfig)
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

	// do not use http.Request
	// since Content-Length requires a Body of same size
	_, err = c.readChan.Write([]byte(
		"GET / HTTP/1.1\r\n" +
			"Host: " + addr + "\r\n" +
			"X-Sessioncookie: " + tunnelID + "\r\n" +
			"Accept: application/x-rtsp-tunnelled\r\n" +
			"Content-Length: 30000\r\n" +
			"\r\n",
	))
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

	c.writeChan, err = dialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	if tlsConfig != nil {
		c.writeChan = tls.Client(c.writeChan, tlsConfig)
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

	// do not use http.Request
	// since Content-Length requires a Body of same size
	_, err = c.writeChan.Write([]byte(
		"POST / HTTP/1.1\r\n" +
			"Host: " + addr + "\r\n" +
			"X-Sessioncookie: " + tunnelID + "\r\n" +
			"Content-Type: application/x-rtsp-tunnelled\r\n" +
			"Content-Length: 30000\r\n" +
			"\r\n",
	))
	if err != nil {
		return nil, err
	}

	writeBuf := bufio.NewReader(c.writeChan)
	res, err = http.ReadResponse(writeBuf, nil)
	if err != nil {
		return nil, err
	}
	res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %v", res.StatusCode)
	}

	ok = true
	return c, nil
}
