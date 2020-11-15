package base

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

func contentRead(rb *bufio.Reader, header Header) ([]byte, error) {
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

func contentWrite(bw *bufio.Writer, content []byte) error {
	if len(content) == 0 {
		return nil
	}

	_, err := bw.Write(content)
	if err != nil {
		return err
	}

	return nil
}
