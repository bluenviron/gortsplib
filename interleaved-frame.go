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

// ConvChannelToTrackIdAndStreamType converts a channel into a track id and a streamType.
func ConvChannelToTrackIdAndStreamType(channel uint8) (int, StreamType) {
	if (channel % 2) == 0 {
		return int(channel / 2), StreamTypeRtp
	}
	return int((channel - 1) / 2), StreamTypeRtcp
}

// ConvTrackIdAndStreamTypeToChannel converts a track id and a streamType into a channel.
func ConvTrackIdAndStreamTypeToChannel(trackId int, StreamType StreamType) uint8 {
	if StreamType == StreamTypeRtp {
		return uint8(trackId * 2)
	}
	return uint8((trackId * 2) + 1)
}

// InterleavedFrame is a structure that allows to send and receive binary data
// within RTSP connections. It is usually deployed to send RTP and RTCP packets via TCP.
type InterleavedFrame struct {
	Channel uint8
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

	f.Channel = header[1]
	f.Content = f.Content[:framelen]

	_, err = io.ReadFull(r, f.Content)
	if err != nil {
		return err
	}

	return nil
}

func (f *InterleavedFrame) write(bw *bufio.Writer) error {
	_, err := bw.Write([]byte{0x24, f.Channel})
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
