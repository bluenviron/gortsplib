package base

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const (
	rtspMaxContentLength = 128 * 1024
)

type body []byte

func (b *body) read(header Header, rb *bufio.Reader) error {
	cls, ok := header["Content-Length"]
	if !ok || len(cls) != 1 {
		*b = nil
		return nil
	}

	cl, err := strconv.ParseInt(cls[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Content-Length")
	}

	if cl > rtspMaxContentLength {
		return fmt.Errorf("Content-Length exceeds %d (it's %d)",
			rtspMaxContentLength, cl)
	}

	*b = make([]byte, cl)
	n, err := io.ReadFull(rb, *b)
	if err != nil && n != len(*b) {
		return err
	}

	return nil
}

func (b body) writeSize() int {
	return len(b)
}

func (b body) writeTo(buf []byte) int {
	return copy(buf, b)
}

func (b body) write() []byte {
	buf := make([]byte, b.writeSize())
	b.writeTo(buf)
	return buf
}
