package base

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
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

func (b body) write(bb *bytes.Buffer) {
	if len(b) == 0 {
		return
	}

	bb.Write(b)
}
