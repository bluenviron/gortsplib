// Package base contains the base elements of the RTSP protocol.
package base

import (
	"bufio"
	"fmt"
	"strconv"
)

const (
	rtspProtocol10           = "RTSP/1.0"
	requestMaxLethodLength   = 128
	requestMaxPathLength     = 1024
	requestMaxProtocolLength = 128
)

// Method is the method of a RTSP request.
type Method string

// standard methods
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
	URL *URL

	// map of header values
	Header Header

	// optional content
	Content []byte

	// whether to wait for a response or not
	// used only by ConnClient.Do()
	SkipResponse bool
}

// Read reads a request.
func (req *Request) Read(rb *bufio.Reader) error {
	byts, err := readBytesLimited(rb, ' ', requestMaxLethodLength)
	if err != nil {
		return err
	}
	req.Method = Method(byts[:len(byts)-1])

	if req.Method == "" {
		return fmt.Errorf("empty method")
	}

	byts, err = readBytesLimited(rb, ' ', requestMaxPathLength)
	if err != nil {
		return err
	}
	rawUrl := string(byts[:len(byts)-1])

	if rawUrl == "" {
		return fmt.Errorf("empty url")
	}

	ur, err := ParseURL(rawUrl)
	if err != nil {
		return fmt.Errorf("unable to parse url (%v)", rawUrl)
	}
	req.URL = ur

	byts, err = readBytesLimited(rb, '\r', requestMaxProtocolLength)
	if err != nil {
		return err
	}
	proto := string(byts[:len(byts)-1])

	if proto != rtspProtocol10 {
		return fmt.Errorf("expected '%s', got '%s'", rtspProtocol10, proto)
	}

	err = readByteEqual(rb, '\n')
	if err != nil {
		return err
	}

	req.Header = make(Header)
	err = req.Header.read(rb)
	if err != nil {
		return err
	}

	req.Content, err = contentRead(rb, req.Header)
	if err != nil {
		return err
	}

	return nil
}

// Write writes a request.
func (req Request) Write(bw *bufio.Writer) error {
	urStr := req.URL.CloneWithoutCredentials().String()
	_, err := bw.Write([]byte(string(req.Method) + " " + urStr + " " + rtspProtocol10 + "\r\n"))
	if err != nil {
		return err
	}

	if len(req.Content) != 0 {
		req.Header["Content-Length"] = HeaderValue{strconv.FormatInt(int64(len(req.Content)), 10)}
	}

	err = req.Header.write(bw)
	if err != nil {
		return err
	}

	err = contentWrite(bw, req.Content)
	if err != nil {
		return err
	}

	return bw.Flush()
}
