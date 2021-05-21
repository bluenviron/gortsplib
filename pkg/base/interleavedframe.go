package base

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	interleavedFrameMagicByte = 0x24
)

// ReadInterleavedFrameOrRequest reads an InterleavedFrame or a Response.
func ReadInterleavedFrameOrRequest(frame *InterleavedFrame, req *Request, br *bufio.Reader) (interface{}, error) {
	b, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	br.UnreadByte()

	if b == interleavedFrameMagicByte {
		err := frame.Read(br)
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
func ReadInterleavedFrameOrResponse(frame *InterleavedFrame, res *Response, br *bufio.Reader) (interface{}, error) {
	b, err := br.ReadByte()
	if err != nil {
		return nil, err
	}
	br.UnreadByte()

	if b == interleavedFrameMagicByte {
		err := frame.Read(br)
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
	// track id
	TrackID int

	// stream type
	StreamType StreamType

	// frame payload
	Payload []byte
}

// Read reads an interleaved frame.
func (f *InterleavedFrame) Read(br *bufio.Reader) error {
	var header [4]byte
	_, err := io.ReadFull(br, header[:])
	if err != nil {
		return err
	}

	if header[0] != interleavedFrameMagicByte {
		return fmt.Errorf("invalid magic byte (0x%.2x)", header[0])
	}

	framelen := int(binary.BigEndian.Uint16(header[2:]))
	if framelen > len(f.Payload) {
		return fmt.Errorf("payload size greater than maximum allowed (%d vs %d)",
			framelen, len(f.Payload))
	}

	// convert channel into TrackID and StreamType
	channel := header[1]
	f.TrackID, f.StreamType = func() (int, StreamType) {
		if (channel % 2) == 0 {
			return int(channel / 2), StreamTypeRTP
		}
		return int((channel - 1) / 2), StreamTypeRTCP
	}()

	f.Payload = f.Payload[:framelen]

	_, err = io.ReadFull(br, f.Payload)
	if err != nil {
		return err
	}
	return nil
}

// Write writes an InterleavedFrame into a buffered writer.
func (f InterleavedFrame) Write(bw *bufio.Writer) error {
	// convert TrackID and StreamType into channel
	channel := func() uint8 {
		if f.StreamType == StreamTypeRTP {
			return uint8(f.TrackID * 2)
		}
		return uint8((f.TrackID * 2) + 1)
	}()

	buf := []byte{0x24, channel, 0x00, 0x00}
	binary.BigEndian.PutUint16(buf[2:], uint16(len(f.Payload)))

	_, err := bw.Write(buf)
	if err != nil {
		return err
	}

	_, err = bw.Write(f.Payload)
	if err != nil {
		return err
	}

	return bw.Flush()
}
