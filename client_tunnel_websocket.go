package gortsplib

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

type clientTunnelWebSocket struct {
	wconn *websocket.Conn
	r     io.Reader
	w     io.Writer
}

func (tu *clientTunnelWebSocket) Read(b []byte) (int, error) {
	return tu.r.Read(b)
}

func (tu *clientTunnelWebSocket) Write(b []byte) (int, error) {
	return tu.w.Write(b)
}

func (tu *clientTunnelWebSocket) Close() error {
	return tu.wconn.Close()
}

func (tu *clientTunnelWebSocket) LocalAddr() net.Addr {
	return tu.wconn.LocalAddr()
}

func (tu *clientTunnelWebSocket) RemoteAddr() net.Addr {
	return tu.wconn.RemoteAddr()
}

func (tu *clientTunnelWebSocket) SetDeadline(_ time.Time) error {
	return nil
}

func (tu *clientTunnelWebSocket) SetReadDeadline(t time.Time) error {
	return tu.wconn.SetReadDeadline(t)
}

func (tu *clientTunnelWebSocket) SetWriteDeadline(t time.Time) error {
	return tu.wconn.SetWriteDeadline(t)
}

func newClientTunnelWebSocket(
	ctx context.Context,
	addr string,
	secure bool,
	tlsConfig *tls.Config,
	dialContext func(ctx context.Context, network, address string) (net.Conn, error),
	dialTLSContext func(ctx context.Context, network string, addr string) (net.Conn, error),
	u *base.URL,
	onHTTPRequest ClientOnHTTPRequestFunc,
) (net.Conn, error) {
	c := &clientTunnelWebSocket{}

	scheme := "ws"
	if secure {
		scheme = "wss"
	}

	requestURL := clientTunnelHTTPRequestURL(u)
	requestURL.Scheme = scheme
	requestURL.Host = addr

	req := &http.Request{
		Method: http.MethodGet,
		URL:    requestURL,
		Host:   addr,
		Header: http.Header{},
	}

	onHTTPRequest(req)

	if req.URL == nil {
		return nil, fmt.Errorf("HTTP request has no URL")
	}

	dialer := &websocket.Dialer{
		Subprotocols: []string{"rtsp.onvif.org"},
	}

	if secure && dialTLSContext != nil {
		dialer.NetDialTLSContext = dialTLSContext
	} else {
		dialer.NetDialContext = dialContext
		dialer.TLSClientConfig = tlsConfig
	}

	var err error
	c.wconn, _, err = dialer.DialContext(ctx, req.URL.String(), req.Header) //nolint:bodyclose
	if err != nil {
		return nil, err
	}

	c.r = &wsReader{wc: c.wconn}
	c.w = &wsWriter{wc: c.wconn}

	return c, nil
}
