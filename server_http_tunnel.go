package gortsplib

import (
	"bufio"
	"io"
	"net"
	"time"

	"github.com/bluenviron/gortsplib/v5/internal/base64streamreader"
)

type serverHTTPTunnel struct {
	r  net.Conn
	rb io.Reader
	w  net.Conn
}

func (m *serverHTTPTunnel) Read(p []byte) (int, error) {
	return m.rb.Read(p)
}

func (m *serverHTTPTunnel) Write(p []byte) (int, error) {
	return m.w.Write(p)
}

func (m *serverHTTPTunnel) Close() error {
	m.r.Close()
	m.w.Close()
	return nil
}

func (m *serverHTTPTunnel) LocalAddr() net.Addr {
	return m.r.LocalAddr()
}

func (m *serverHTTPTunnel) RemoteAddr() net.Addr {
	return m.r.RemoteAddr()
}

func (m *serverHTTPTunnel) SetDeadline(_ time.Time) error {
	panic("unimplemented")
}

func (m *serverHTTPTunnel) SetReadDeadline(t time.Time) error {
	return m.r.SetReadDeadline(t)
}

func (m *serverHTTPTunnel) SetWriteDeadline(t time.Time) error {
	return m.w.SetWriteDeadline(t)
}

func newServerHTTPTunnel(r net.Conn, rb *bufio.Reader, w net.Conn) net.Conn {
	return &serverHTTPTunnel{
		r:  r,
		rb: base64streamreader.New(rb),
		w:  w,
	}
}
