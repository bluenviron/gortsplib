package gortsplib

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsNetConn struct {
	r   io.Reader
	buf *bufio.Reader
	w   io.Writer
}

func (c *wsNetConn) Read(b []byte) (n int, err error) {
	return c.r.Read(b)
}

func (c *wsNetConn) Write(b []byte) (n int, err error) {
	return c.w.Write(b)
}

func (c *wsNetConn) Close() error {
	panic("unimplemented")
}

func (c *wsNetConn) LocalAddr() net.Addr {
	panic("unimplemented")
}

func (c *wsNetConn) RemoteAddr() net.Addr {
	panic("unimplemented")
}

func (c *wsNetConn) SetDeadline(_ time.Time) error {
	return nil
}

func (c *wsNetConn) SetReadDeadline(_ time.Time) error {
	return nil
}

func (c *wsNetConn) SetWriteDeadline(_ time.Time) error {
	return nil
}

type wsResponseWriter struct {
	r   io.Reader
	buf *bufio.Reader
	w   io.Writer
	req *http.Request

	h http.Header
}

func (w *wsResponseWriter) initialize() {
	w.h = make(http.Header)
}

func (w *wsResponseWriter) Header() http.Header {
	return w.h
}

func (w *wsResponseWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

func (w *wsResponseWriter) WriteHeader(statusCode int) {
	res := http.Response{
		StatusCode: statusCode,
		ProtoMajor: w.req.ProtoMajor,
		ProtoMinor: w.req.ProtoMinor,
		Header:     w.h,
		Request:    w.req,
	}
	var buf2 bytes.Buffer
	res.Write(&buf2) //nolint:errcheck
	w.w.Write(buf2.Bytes())
}

func (w *wsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return &wsNetConn{r: w.r, buf: w.buf, w: w.w}, bufio.NewReadWriter(w.buf, bufio.NewWriter(w.w)), nil
}

type wsReader struct {
	wc *websocket.Conn

	buf []byte
}

func (r *wsReader) Read(p []byte) (int, error) {
	if len(r.buf) == 0 {
		var msgType int
		var err error
		msgType, r.buf, err = r.wc.ReadMessage()
		if err != nil {
			return 0, err
		}

		if msgType != websocket.BinaryMessage {
			return 0, fmt.Errorf("unxpected message type %v", msgType)
		}
	}

	n := copy(p, r.buf)
	r.buf = r.buf[n:]

	return n, nil
}

type wsWriter struct {
	wc *websocket.Conn

	mutex sync.Mutex
}

func (w *wsWriter) Write(p []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	err := w.wc.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
