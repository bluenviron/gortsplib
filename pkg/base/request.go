// Package base contains the primitives of the RTSP protocol.
package base

import (
	"bufio"
	"fmt"
	"strconv"
)

const (
	rtspProtocol10           = "RTSP/1.0"
	requestMaxMethodLength   = 64
	requestMaxURLLength      = 2048
	requestMaxProtocolLength = 64
)

// Method is the method of a RTSP request.
type Method string

// methods.
const (
	Announce     Method = "ANNOUNCE"
	Describe     Method = "DESCRIBE"
	GetParameter Method = "GET_PARAMETER"
	Options      Method = "OPTIONS"
	Pause        Method = "PAUSE"
	Play         Method = "PLAY"
	Record       Method = "RECORD"
	Setup        Method = "SETUP"
	SetParameter Method = "SET_PARAMETER"
	Teardown     Method = "TEARDOWN"
)

// Request is a RTSP request.
type Request struct {
	// request method
	Method Method

	// request url
	URL *URL

	// map of header values
	Header Header

	// optional body
	Body []byte
}

// Read reads a request.
func (req *Request) Read(rb *bufio.Reader) error {
	byts, err := readBytesLimited(rb, ' ', requestMaxMethodLength)
	if err != nil {
		return err
	}
	req.Method = Method(byts[:len(byts)-1])

	if req.Method == "" {
		return fmt.Errorf("empty method")
	}

	byts, err = readBytesLimited(rb, ' ', requestMaxURLLength)
	if err != nil {
		return err
	}
	rawURL := string(byts[:len(byts)-1])

	ur, err := ParseURL(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL (%v)", rawURL)
	}
	req.URL = ur

	byts, err = readBytesLimited(rb, '\r', requestMaxProtocolLength)
	if err != nil {
		return err
	}
	proto := byts[:len(byts)-1]

	if string(proto) != rtspProtocol10 {
		return fmt.Errorf("expected '%s', got %v", rtspProtocol10, proto)
	}

	err = readByteEqual(rb, '\n')
	if err != nil {
		return err
	}

	err = req.Header.read(rb)
	if err != nil {
		return err
	}

	err = (*body)(&req.Body).read(req.Header, rb)
	if err != nil {
		return err
	}

	return nil
}

// ReadIgnoreFrames reads a request and ignores any interleaved frame sent
// before the request.
func (req *Request) ReadIgnoreFrames(maxPayloadSize int, rb *bufio.Reader) error {
	var f InterleavedFrame

	for {
		recv, err := ReadInterleavedFrameOrRequest(&f, maxPayloadSize, req, rb)
		if err != nil {
			return err
		}

		if _, ok := recv.(*Request); ok {
			return nil
		}
	}
}

// WriteSize returns the size of a Request.
func (req Request) WriteSize() int {
	n := 0

	urStr := req.URL.CloneWithoutCredentials().String()
	n += len([]byte(string(req.Method) + " " + urStr + " " + rtspProtocol10 + "\r\n"))

	if len(req.Body) != 0 {
		req.Header["Content-Length"] = HeaderValue{strconv.FormatInt(int64(len(req.Body)), 10)}
	}

	n += req.Header.writeSize()

	n += body(req.Body).writeSize()

	return n
}

// WriteTo writes a Request.
func (req Request) WriteTo(buf []byte) (int, error) {
	pos := 0

	urStr := req.URL.CloneWithoutCredentials().String()
	pos += copy(buf[pos:], []byte(string(req.Method)+" "+urStr+" "+rtspProtocol10+"\r\n"))

	if len(req.Body) != 0 {
		req.Header["Content-Length"] = HeaderValue{strconv.FormatInt(int64(len(req.Body)), 10)}
	}

	pos += req.Header.writeTo(buf[pos:])

	pos += body(req.Body).writeTo(buf[pos:])

	return pos, nil
}

// Write writes a Request.
func (req Request) Write() ([]byte, error) {
	buf := make([]byte, req.WriteSize())
	_, err := req.WriteTo(buf)
	return buf, err
}

// String implements fmt.Stringer.
func (req Request) String() string {
	buf, _ := req.Write()
	return string(buf)
}
