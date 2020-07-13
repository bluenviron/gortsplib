package gortsplib

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	interleavedFrameMagicByte = 0x24
)

// StreamType is the type of a stream.
type StreamType int

const (
	// StreamTypeRtp is a stream that contains RTP packets
	StreamTypeRtp StreamType = iota

	// StreamTypeRtcp is a stream that contains RTCP packets
	StreamTypeRtcp
)

func (st StreamType) String() string {
	switch st {
	case StreamTypeRtp:
		return "RTP"

	case StreamTypeRtcp:
		return "RTCP"
	}
	return "UNKNOWN"
}

// InterleavedFrame is an object that allows to send and receive binary data
// within RTSP connections. It is used to send RTP and RTCP packets via TCP.
type InterleavedFrame struct {
	// track id
	TrackId int

	// stream type
	StreamType StreamType

	// frame content
	Content []byte
}

func (f *InterleavedFrame) read(r io.Reader) error {
	var header [4]byte
	_, err := io.ReadFull(r, header[:])
	if err != nil {
		return err
	}

	if header[0] != interleavedFrameMagicByte {
		return fmt.Errorf("wrong magic byte (0x%.2x)", header[0])
	}

	framelen := int(binary.BigEndian.Uint16(header[2:]))
	if framelen > len(f.Content) {
		return fmt.Errorf("frame length greater than maximum allowed (%d vs %d)",
			framelen, len(f.Content))
	}

	// convert channel into TrackId and StreamType
	channel := header[1]
	f.TrackId, f.StreamType = func() (int, StreamType) {
		if (channel % 2) == 0 {
			return int(channel / 2), StreamTypeRtp
		}
		return int((channel - 1) / 2), StreamTypeRtcp
	}()

	f.Content = f.Content[:framelen]

	_, err = io.ReadFull(r, f.Content)
	if err != nil {
		return err
	}
	return nil
}

func (f *InterleavedFrame) write(bw *bufio.Writer) error {
	// convert TrackId and StreamType into channel
	channel := func() uint8 {
		if f.StreamType == StreamTypeRtp {
			return uint8(f.TrackId * 2)
		}
		return uint8((f.TrackId * 2) + 1)
	}()

	_, err := bw.Write([]byte{0x24, channel})
	if err != nil {
		return err
	}

	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, uint16(len(f.Content)))
	_, err = bw.Write(buf)
	if err != nil {
		return err
	}

	_, err = bw.Write(f.Content)
	if err != nil {
		return err
	}

	return bw.Flush()
}
