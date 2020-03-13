package gortsplib

import (
	"bufio"
	"fmt"
)

const (
	_MAX_METHOD_LENGTH   = 128
	_MAX_PATH_LENGTH     = 1024
	_MAX_PROTOCOL_LENGTH = 128
)

// Request is a RTSP request.
type Request struct {
	Method  string
	Url     string
	Header  Header
	Content []byte
}

func readRequest(br *bufio.Reader) (*Request, error) {
	req := &Request{}

	byts, err := readBytesLimited(br, ' ', _MAX_METHOD_LENGTH)
	if err != nil {
		return nil, err
	}
	req.Method = string(byts[:len(byts)-1])

	if len(req.Method) == 0 {
		return nil, fmt.Errorf("empty method")
	}

	byts, err = readBytesLimited(br, ' ', _MAX_PATH_LENGTH)
	if err != nil {
		return nil, err
	}
	req.Url = string(byts[:len(byts)-1])

	if len(req.Url) == 0 {
		return nil, fmt.Errorf("empty url")
	}

	byts, err = readBytesLimited(br, '\r', _MAX_PROTOCOL_LENGTH)
	if err != nil {
		return nil, err
	}
	proto := string(byts[:len(byts)-1])

	if proto != _RTSP_PROTO {
		return nil, fmt.Errorf("expected '%s', got '%s'", _RTSP_PROTO, proto)
	}

	err = readByteEqual(br, '\n')
	if err != nil {
		return nil, err
	}

	req.Header, err = readHeader(br)
	if err != nil {
		return nil, err
	}

	req.Content, err = readContent(br, req.Header)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (req *Request) write(bw *bufio.Writer) error {
	_, err := bw.Write([]byte(req.Method + " " + req.Url + " " + _RTSP_PROTO + "\r\n"))
	if err != nil {
		return err
	}

	err = req.Header.write(bw)
	if err != nil {
		return err
	}

	err = writeContent(bw, req.Content)
	if err != nil {
		return err
	}

	return bw.Flush()
}
