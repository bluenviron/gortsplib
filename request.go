package gortsplib

import (
	"bufio"
	"fmt"
	"net/url"
)

const (
	_MAX_METHOD_LENGTH   = 128
	_MAX_PATH_LENGTH     = 1024
	_MAX_PROTOCOL_LENGTH = 128
)

// Method is a RTSP request method.
type Method string

const (
	ANNOUNCE      Method = "ANNOUNCE"
	DESCRIBE      Method = "DESCRIBE"
	GET_PARAMETER Method = "GET_PARAMETER"
	OPTIONS       Method = "OPTIONS"
	PAUSE         Method = "PAUSE"
	PLAY          Method = "PLAY"
	PLAY_NOTIFY   Method = "PLAY_NOTIFY"
	RECORD        Method = "RECORD"
	REDIRECT      Method = "REDIRECT"
	SETUP         Method = "SETUP"
	SET_PARAMETER Method = "SET_PARAMETER"
	TEARDOWN      Method = "TEARDOWN"
)

// Request is a RTSP request.
type Request struct {
	Method  Method
	Url     *url.URL
	Header  Header
	Content []byte
}

func readRequest(br *bufio.Reader) (*Request, error) {
	req := &Request{}

	byts, err := readBytesLimited(br, ' ', _MAX_METHOD_LENGTH)
	if err != nil {
		return nil, err
	}
	req.Method = Method(byts[:len(byts)-1])

	if req.Method == "" {
		return nil, fmt.Errorf("empty method")
	}

	byts, err = readBytesLimited(br, ' ', _MAX_PATH_LENGTH)
	if err != nil {
		return nil, err
	}
	rawUrl := string(byts[:len(byts)-1])

	if rawUrl == "" {
		return nil, fmt.Errorf("empty url")
	}

	ur, err := url.Parse(rawUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to parse url '%s'", rawUrl)
	}
	req.Url = ur

	if req.Url.Scheme != "rtsp" {
		return nil, fmt.Errorf("invalid url scheme '%s'", req.Url.Scheme)
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
	_, err := bw.Write([]byte(string(req.Method) + " " + req.Url.String() + " " + _RTSP_PROTO + "\r\n"))
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
