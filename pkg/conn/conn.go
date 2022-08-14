package conn

import (
	"bufio"
	"io"

	"github.com/aler9/gortsplib/pkg/base"
)

const (
	readBufferSize = 4096
)

// Conn is a RTSP TCP connection.
type Conn struct {
	w  io.Writer
	br *bufio.Reader
}

// NewConn allocates a Conn.
func NewConn(rw io.ReadWriter) *Conn {
	return &Conn{
		w:  rw,
		br: bufio.NewReaderSize(rw, readBufferSize),
	}
}

// ReadResponse reads a Response.
func (c *Conn) ReadResponse(res *base.Response) error {
	return res.Read(c.br)
}

// ReadRequest reads a Request.
func (c *Conn) ReadRequest(req *base.Request) error {
	return req.Read(c.br)
}

// ReadInterleavedFrame reads a InterleavedFrame.
func (c *Conn) ReadInterleavedFrame(fr *base.InterleavedFrame) error {
	return fr.Read(c.br)
}

// ReadInterleavedFrameOrRequest reads an InterleavedFrame or a Request.
func (c *Conn) ReadInterleavedFrameOrRequest(
	frame *base.InterleavedFrame,
	req *base.Request,
) (interface{}, error) {
	b, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	c.br.UnreadByte()

	if b == base.InterleavedFrameMagicByte {
		err := frame.Read(c.br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	err = req.Read(c.br)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// ReadInterleavedFrameOrResponse reads an InterleavedFrame or a Response.
func (c *Conn) ReadInterleavedFrameOrResponse(
	frame *base.InterleavedFrame,
	res *base.Response,
) (interface{}, error) {
	b, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	c.br.UnreadByte()

	if b == base.InterleavedFrameMagicByte {
		err := frame.Read(c.br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	err = res.Read(c.br)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// ReadRequestIgnoreFrames reads a Request and ignore frames in between.
func (c *Conn) ReadRequestIgnoreFrames(req *base.Request) error {
	var f base.InterleavedFrame

	for {
		recv, err := c.ReadInterleavedFrameOrRequest(&f, req)
		if err != nil {
			return err
		}

		if _, ok := recv.(*base.Request); ok {
			return nil
		}
	}
}

// ReadResponseIgnoreFrames reads a Response and ignore frames in between.
func (c *Conn) ReadResponseIgnoreFrames(res *base.Response) error {
	var f base.InterleavedFrame

	for {
		recv, err := c.ReadInterleavedFrameOrResponse(&f, res)
		if err != nil {
			return err
		}

		if _, ok := recv.(*base.Response); ok {
			return nil
		}
	}
}

// WriteRequest writes a request.
func (c *Conn) WriteRequest(req *base.Request) error {
	buf, _ := req.Marshal()
	_, err := c.w.Write(buf)
	return err
}

// WriteResponse writes a response.
func (c *Conn) WriteResponse(res *base.Response) error {
	buf, _ := res.Marshal()
	_, err := c.w.Write(buf)
	return err
}

// WriteInterleavedFrame writes an interleaved frame.
func (c *Conn) WriteInterleavedFrame(fr *base.InterleavedFrame, buf []byte) error {
	n, _ := fr.MarshalTo(buf)
	_, err := c.w.Write(buf[:n])
	return err
}
