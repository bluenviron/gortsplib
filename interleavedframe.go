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

// InterleavedFrame is a structure that allows to transfer binary data
// within RTSP/TCP connections. It is used to send and receive RTP and RTCP packets with TCP.
type InterleavedFrame struct {
	// track id
	TrackId int

	// stream type
	StreamType StreamType

	// frame content
	Content []byte
}

// Read reads an interleaved frame from a buffered reader.
func (f *InterleavedFrame) Read(br *bufio.Reader) error {
	var header [4]byte
	_, err := io.ReadFull(br, header[:])
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

	_, err = io.ReadFull(br, f.Content)
	if err != nil {
		return err
	}
	return nil
}

// Write writes an InterleavedFrame into a buffered writer.
func (f *InterleavedFrame) Write(bw *bufio.Writer) error {
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
