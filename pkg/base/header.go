package base

import (
	"bufio"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

const (
	headerMaxEntryCount  = 255
	headerMaxKeyLength   = 512
	headerMaxValueLength = 2048
)

func headerKeyNormalize(in string) string {
	switch strings.ToLower(in) {
	case "rtp-info":
		return "RTP-Info"

	case "www-authenticate":
		return "WWW-Authenticate"

	case "cseq":
		return "CSeq"
	}
	return http.CanonicalHeaderKey(in)
}

// HeaderValue is an header value.
type HeaderValue []string

// Header is a RTSP reader, present in both Requests and Responses.
type Header map[string]HeaderValue

func (h *Header) read(rb *bufio.Reader) error {
	*h = make(Header)

	for {
		byt, err := rb.ReadByte()
		if err != nil {
			return err
		}

		if byt == '\r' {
			err := readByteEqual(rb, '\n')
			if err != nil {
				return err
			}

			break
		}

		if len(*h) >= headerMaxEntryCount {
			return fmt.Errorf("headers count exceeds %d (it's %d)",
				headerMaxEntryCount, len(*h))
		}

		key := string([]byte{byt})
		byts, err := readBytesLimited(rb, ':', headerMaxKeyLength-1)
		if err != nil {
			return err
		}
		key += string(byts[:len(byts)-1])
		key = headerKeyNormalize(key)

		// https://tools.ietf.org/html/rfc2616
		// The field value MAY be preceded by any amount of spaces
		for {
			byt, err := rb.ReadByte()
			if err != nil {
				return err
			}

			if byt != ' ' {
				break
			}
		}
		rb.UnreadByte()

		byts, err = readBytesLimited(rb, '\r', headerMaxValueLength)
		if err != nil {
			return err
		}
		val := string(byts[:len(byts)-1])

		if len(val) == 0 {
			return fmt.Errorf("empty header value")
		}

		err = readByteEqual(rb, '\n')
		if err != nil {
			return err
		}

		(*h)[key] = append((*h)[key], val)
	}

	return nil
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
