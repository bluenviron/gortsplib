package base

import (
	"bufio"
	"fmt"
	"io"
)

const (
	interleavedFrameMagicByte = 0x24
)

// ReadInterleavedFrameOrRequest reads an InterleavedFrame or a Response.
func ReadInterleavedFrameOrRequest(
	frame *InterleavedFrame,
	maxPayloadSize int,
	req *Request,
	br *bufio.Reader,
) (interface{}, error) {
	b, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	br.UnreadByte()

	if b == interleavedFrameMagicByte {
		err := frame.Read(maxPayloadSize, br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	err = req.Read(br)
	if err != nil {
		return nil, err
	}
	return req, nil
}

// ReadInterleavedFrameOrResponse reads an InterleavedFrame or a Response.
func ReadInterleavedFrameOrResponse(
	frame *InterleavedFrame,
	maxPayloadSize int,
	res *Response,
	br *bufio.Reader,
) (interface{}, error) {
	b, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	br.UnreadByte()

	if b == interleavedFrameMagicByte {
		err := frame.Read(maxPayloadSize, br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	err = res.Read(br)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// InterleavedFrame is an interleaved frame, and allows to transfer binary data
// within RTSP/TCP connections. It is used to send and receive RTP and RTCP packets with TCP.
type InterleavedFrame struct {
	// channel ID
	Channel int

	// payload
	Payload []byte
}

// Read decodes an interleaved frame.
func (f *InterleavedFrame) Read(maxPayloadSize int, br *bufio.Reader) error {
	var header [4]byte
	_, err := io.ReadFull(br, header[:])
	if err != nil {
		return err
	}

	if header[0] != interleavedFrameMagicByte {
		return fmt.Errorf("invalid magic byte (0x%.2x)", header[0])
	}

	payloadLen := int(uint16(header[2])<<8 | uint16(header[3]))
	if payloadLen > maxPayloadSize {
		return fmt.Errorf("payload size (%d) greater than maximum allowed (%d)",
			payloadLen, maxPayloadSize)
	}

	f.Channel = int(header[1])
	f.Payload = make([]byte, payloadLen)

	_, err = io.ReadFull(br, f.Payload)
	if err != nil {
		return err
	}
	return nil
}

// MarshalSize returns the size of an InterleavedFrame.
func (f InterleavedFrame) MarshalSize() int {
	return 4 + len(f.Payload)
}

// MarshalTo writes an InterleavedFrame.
func (f InterleavedFrame) MarshalTo(buf []byte) (int, error) {
	pos := 0

	pos += copy(buf[pos:], []byte{0x24, byte(f.Channel)})

	payloadLen := len(f.Payload)
	buf[pos] = byte(payloadLen >> 8)
	buf[pos+1] = byte(payloadLen)
	pos += 2

	pos += copy(buf[pos:], f.Payload)

	return pos, nil
}

// Marshal writes an InterleavedFrame.
func (f InterleavedFrame) Marshal() ([]byte, error) {
	buf := make([]byte, f.MarshalSize())
	_, err := f.MarshalTo(buf)
	return buf, err
}
