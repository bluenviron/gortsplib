package base

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const (
	rtspMaxContentLength = 4096
)

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
