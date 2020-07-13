package gortsplib

import (
	"bufio"
	"fmt"
	"net/url"
)

const (
	rtspProtocol10           = "RTSP/1.0"
	requestMaxLethodLength   = 128
	requestMaxPathLength     = 1024
	requestMaxProtocolLength = 128
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
	// request method
	Method Method

	// request url
	Url *url.URL

	// map of header values
	Header Header

	// optional content
	Content []byte

	// whether to wait for a response or not
	// used only by ConnClient.Do()
	SkipResponse bool
}

func readRequest(rb *bufio.Reader) (*Request, error) {
	req := &Request{}

	byts, err := readBytesLimited(rb, ' ', requestMaxLethodLength)
	if err != nil {
		return nil, err
	}
	req.Method = Method(byts[:len(byts)-1])

	if req.Method == "" {
		return nil, fmt.Errorf("empty method")
	}

	byts, err = readBytesLimited(rb, ' ', requestMaxPathLength)
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

	byts, err = readBytesLimited(rb, '\r', requestMaxProtocolLength)
	if err != nil {
		return nil, err
	}
	proto := string(byts[:len(byts)-1])

	if proto != rtspProtocol10 {
		return nil, fmt.Errorf("expected '%s', got '%s'", rtspProtocol10, proto)
	}

	err = readByteEqual(rb, '\n')
	if err != nil {
		return nil, err
	}

	req.Header, err = headerRead(rb)
	if err != nil {
		return nil, err
	}

	req.Content, err = readContent(rb, req.Header)
	if err != nil {
		return nil, err
	}

	return req, nil
}

func (req *Request) write(bw *bufio.Writer) error {
	// remove credentials
	u := &url.URL{
		Scheme:   req.Url.Scheme,
		Host:     req.Url.Host,
		Path:     req.Url.Path,
		RawQuery: req.Url.RawQuery,
	}

	_, err := bw.Write([]byte(string(req.Method) + " " + u.String() + " " + rtspProtocol10 + "\r\n"))
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
