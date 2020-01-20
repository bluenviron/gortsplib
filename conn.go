package gortsplib

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	_READ_DEADLINE  = 10 * time.Second
	_WRITE_DEADLINE = 10 * time.Second
)

type Conn struct {
	nconn    net.Conn
	writeBuf []byte
}

func NewConn(nconn net.Conn) *Conn {
	return &Conn{
		nconn:    nconn,
		writeBuf: make([]byte, 2048),
	}
}

func (c *Conn) ReadRequest() (*Request, error) {
	c.nconn.SetReadDeadline(time.Now().Add(_READ_DEADLINE))
	return requestDecode(c.nconn)
}

func (c *Conn) WriteRequest(req *Request) error {
	c.nconn.SetWriteDeadline(time.Now().Add(_WRITE_DEADLINE))
	return requestEncode(c.nconn, req)
}

func (c *Conn) ReadResponse() (*Response, error) {
	c.nconn.SetReadDeadline(time.Now().Add(_READ_DEADLINE))
	return responseDecode(c.nconn)
}

func (c *Conn) WriteResponse(res *Response) error {
	c.nconn.SetWriteDeadline(time.Now().Add(_WRITE_DEADLINE))
	return responseEncode(c.nconn, res)
}

func (c *Conn) ReadInterleavedFrame(frame []byte) (int, int, error) {
	c.nconn.SetReadDeadline(time.Now().Add(_READ_DEADLINE))

	var header [4]byte
	_, err := io.ReadFull(c.nconn, header[:])
	if err != nil {
		return 0, 0, err
	}

	// connection terminated
	if header[0] == 0x54 {
		return 0, 0, io.EOF
	}

	if header[0] != 0x24 {
		return 0, 0, fmt.Errorf("wrong magic byte (0x%.2x)", header[0])
	}

	framelen := binary.BigEndian.Uint16(header[2:])
	if framelen > 2048 {
		return 0, 0, fmt.Errorf("frame length greater than 2048")
	}

	_, err = io.ReadFull(c.nconn, frame[:framelen])
	if err != nil {
		return 0, 0, err
	}

	return int(header[1]), int(framelen), nil
}

func (c *Conn) WriteInterleavedFrame(channel int, frame []byte) error {
	c.nconn.SetWriteDeadline(time.Now().Add(_WRITE_DEADLINE))

	c.writeBuf[0] = 0x24
	c.writeBuf[1] = byte(channel)
	binary.BigEndian.PutUint16(c.writeBuf[2:], uint16(len(frame)))
	n := copy(c.writeBuf[4:], frame)

	_, err := c.nconn.Write(c.writeBuf[:4+n])
	if err != nil {
		return err
	}
	return nil
}
