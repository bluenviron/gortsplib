package base

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

type payload []byte

func (c *payload) read(rb *bufio.Reader, header Header) error {
	cls, ok := header["Content-Length"]
	if !ok || len(cls) != 1 {
		*c = nil
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

	*c = make([]byte, cl)
	n, err := io.ReadFull(rb, *c)
	if err != nil && n != len(*c) {
		return err
	}

	return nil
}

func (c payload) write(bw *bufio.Writer) error {
	if len(c) == 0 {
		return nil
	}

	_, err := bw.Write(c)
	if err != nil {
		return err
	}

	return nil
}
