package gortsplib

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const (
	rtspMaxContentLength = 4096
)

// StreamProtocol is the protocol of a stream
type StreamProtocol int

const (
	// StreamProtocolUDP means that the stream uses the UDP protocol
	StreamProtocolUDP StreamProtocol = iota

	// StreamProtocolTCP means that the stream uses the TCP protocol
	StreamProtocolTCP
)

// String implements fmt.Stringer
func (sp StreamProtocol) String() string {
	if sp == StreamProtocolUDP {
		return "udp"
	}
	return "tcp"
}

// StreamType is the type of a stream.
type StreamType int

const (
	// StreamTypeRtp means that the stream contains RTP packets
	StreamTypeRtp StreamType = iota

	// StreamTypeRtcp means that the stream contains RTCP packets
	StreamTypeRtcp
)

// String implements fmt.Stringer
func (st StreamType) String() string {
	switch st {
	case StreamTypeRtp:
		return "RTP"

	case StreamTypeRtcp:
		return "RTCP"
	}
	return "UNKNOWN"
}

func readBytesLimited(rb *bufio.Reader, delim byte, n int) ([]byte, error) {
	for i := 1; i <= n; i++ {
		byts, err := rb.Peek(i)
		if err != nil {
			return nil, err
		}

		if byts[len(byts)-1] == delim {
			rb.Discard(len(byts))
			return byts, nil
		}
	}
	return nil, fmt.Errorf("buffer length exceeds %d", n)
}

func readByteEqual(rb *bufio.Reader, cmp byte) error {
	byt, err := rb.ReadByte()
	if err != nil {
		return err
	}

	if byt != cmp {
		return fmt.Errorf("expected '%c', got '%c'", cmp, byt)
	}

	return nil
}

func readContent(rb *bufio.Reader, header Header) ([]byte, error) {
	cls, ok := header["Content-Length"]
	if !ok || len(cls) != 1 {
		return nil, nil
	}

	cl, err := strconv.ParseInt(cls[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Length")
	}

	if cl > rtspMaxContentLength {
		return nil, fmt.Errorf("Content-Length exceeds %d", rtspMaxContentLength)
	}

	ret := make([]byte, cl)
	n, err := io.ReadFull(rb, ret)
	if err != nil && n != len(ret) {
		return nil, err
	}

	return ret, nil
}

func writeContent(bw *bufio.Writer, content []byte) error {
	if len(content) == 0 {
		return nil
	}

	_, err := bw.Write(content)
	if err != nil {
		return err
	}

	return nil
}
