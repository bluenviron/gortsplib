package gortsplib

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
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
	dialContext func(ctx context.Context, network, address string) (net.Conn, error),
	addr string,
	tlsConfig *tls.Config,
) (net.Conn, error) {
	c := &clientTunnelWebSocket{}

	var ur string
	if tlsConfig != nil {
		ur = "wss"
	} else {
		ur = "ws"
	}
	ur += "://" + addr + "/"

	var err error
	c.wconn, _, err = (&websocket.Dialer{
		NetDialContext:  dialContext,
		TLSClientConfig: tlsConfig,
		Subprotocols:    []string{"rtsp.onvif.org"},
	}).DialContext(ctx, ur, nil) //nolint:bodyclose
	if err != nil {
		return nil, err
	}

	c.r = &wsReader{wc: c.wconn}
	c.w = &wsWriter{wc: c.wconn}

	return c, nil
}
