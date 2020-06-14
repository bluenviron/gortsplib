package gortsplib

import (
	"bufio"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const (
	_MAX_HEADER_COUNT        = 255
	_MAX_HEADER_KEY_LENGTH   = 1024
	_MAX_HEADER_VALUE_LENGTH = 1024
)

func headerKeyNormalize(in string) string {
	switch strings.ToLower(in) {
	case "rtp-info":
		return "RTP-INFO"

	case "www-authenticate":
		return "WWW-Authenticate"

	case "cseq":
		return "CSeq"
	}
	return http.CanonicalHeaderKey(in)
}

// Header is a RTSP reader, present in both Requests and Responses.
type Header map[string][]string

func headerRead(rb *bufio.Reader) (Header, error) {
	h := make(Header)

	for {
		byt, err := rb.ReadByte()
		if err != nil {
			return nil, err
		}

		if byt == '\r' {
			err := readByteEqual(rb, '\n')
			if err != nil {
				return nil, err
			}

			break
		}

		if len(h) >= _MAX_HEADER_COUNT {
			return nil, fmt.Errorf("headers count exceeds %d", _MAX_HEADER_COUNT)
		}

		key := string([]byte{byt})
		byts, err := readBytesLimited(rb, ':', _MAX_HEADER_KEY_LENGTH-1)
		if err != nil {
			return nil, err
		}
		key += string(byts[:len(byts)-1])
		key = headerKeyNormalize(key)

		// https://tools.ietf.org/html/rfc2616
		// The field value MAY be preceded by any amount of spaces
		for {
			byt, err := rb.ReadByte()
			if err != nil {
				return nil, err
			}

			if byt != ' ' {
				break
			}
		}
		rb.UnreadByte()

		byts, err = readBytesLimited(rb, '\r', _MAX_HEADER_VALUE_LENGTH)
		if err != nil {
			return nil, err
		}
		val := string(byts[:len(byts)-1])

		if len(val) == 0 {
			return nil, fmt.Errorf("empty header value")
		}

		err = readByteEqual(rb, '\n')
		if err != nil {
			return nil, err
		}

		h[key] = append(h[key], val)
	}

	return h, nil
}

func (h Header) write(wb *bufio.Writer) error {
	// sort headers by key
	// in order to obtain deterministic results
	var keys []string
	for key := range h {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		for _, val := range h[key] {
			_, err := wb.Write([]byte(key + ": " + val + "\r\n"))
			if err != nil {
				return err
			}
		}
	}

	_, err := wb.Write([]byte("\r\n"))
	if err != nil {
		return err
	}

	return nil
}
