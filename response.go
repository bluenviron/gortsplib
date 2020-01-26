package gortsplib

import (
	"bufio"
	"fmt"
	"strconv"
)

// Response is a RTSP response.
type Response struct {
	StatusCode int
	Status     string
	Header     Header
	Content    []byte
}

func readResponse(br *bufio.Reader) (*Response, error) {
	res := &Response{}

	byts, err := readBytesLimited(br, ' ', 255)
	if err != nil {
		return nil, err
	}
	proto := string(byts[:len(byts)-1])

	if proto != _RTSP_PROTO {
		return nil, fmt.Errorf("expected '%s', got '%s'", _RTSP_PROTO, proto)
	}

	byts, err = readBytesLimited(br, ' ', 4)
	if err != nil {
		return nil, err
	}
	statusCodeStr := string(byts[:len(byts)-1])

	statusCode64, err := strconv.ParseInt(statusCodeStr, 10, 32)
	res.StatusCode = int(statusCode64)
	if err != nil {
		return nil, fmt.Errorf("unable to parse status code")
	}

	byts, err = readBytesLimited(br, '\r', 255)
	if err != nil {
		return nil, err
	}
	res.Status = string(byts[:len(byts)-1])

	if len(res.Status) == 0 {
		return nil, fmt.Errorf("empty status")
	}

	err = readByteEqual(br, '\n')
	if err != nil {
		return nil, err
	}

	res.Header, err = readHeader(br)
	if err != nil {
		return nil, err
	}

	res.Content, err = readContent(br, res.Header)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (res *Response) write(bw *bufio.Writer) error {
	_, err := bw.Write([]byte(_RTSP_PROTO + " " + strconv.FormatInt(int64(res.StatusCode), 10) + " " + res.Status + "\r\n"))
	if err != nil {
		return err
	}

	if len(res.Content) != 0 {
		res.Header["Content-Length"] = []string{strconv.FormatInt(int64(len(res.Content)), 10)}
	}

	err = res.Header.write(bw)
	if err != nil {
		return err
	}

	err = writeContent(bw, res.Content)
	if err != nil {
		return err
	}

	return bw.Flush()
}
