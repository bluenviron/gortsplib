package gortsplib

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

const (
	_RTSP_PROTO         = "RTSP/1.0"
	_MAX_CONTENT_LENGTH = 4096
)

func readBytesLimited(br *bufio.Reader, delim byte, n int) ([]byte, error) {
	for i := 1; i <= n; i++ {
		byts, err := br.Peek(i)
		if err != nil {
			return nil, err
		}

		if byts[len(byts)-1] == delim {
			br.Discard(len(byts))
			return byts, nil
		}
	}
	return nil, fmt.Errorf("buffer length exceeds %d", n)
}

func readByteEqual(br *bufio.Reader, cmp byte) error {
	byt, err := br.ReadByte()
	if err != nil {
		return err
	}

	if byt != cmp {
		return fmt.Errorf("expected '%c', got '%c'", cmp, byt)
	}

	return nil
}

func readContent(br *bufio.Reader, header Header) ([]byte, error) {
	cls, ok := header["Content-Length"]
	if !ok || len(cls) != 1 {
		return nil, nil
	}

	cl, err := strconv.ParseInt(cls[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid Content-Length")
	}

	if cl > _MAX_CONTENT_LENGTH {
		return nil, fmt.Errorf("Content-Length exceeds %d", _MAX_CONTENT_LENGTH)
	}

	ret := make([]byte, cl)
	n, err := io.ReadFull(br, ret)
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
