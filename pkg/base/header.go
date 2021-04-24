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
	count := 0

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

		if count >= headerMaxEntryCount {
			return fmt.Errorf("headers count exceeds %d", headerMaxEntryCount)
		}

		key := string([]byte{byt})
		byts, err := readBytesLimited(rb, ':', headerMaxKeyLength-1)
		if err != nil {
			return fmt.Errorf("value is missing")
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

		err = readByteEqual(rb, '\n')
		if err != nil {
			return err
		}

		(*h)[key] = append((*h)[key], val)
		count++
	}

	return nil
}

func (h Header) write(wb *bufio.Writer) error {
	// sort headers by key
	// in order to obtain deterministic results
	keys := make([]string, len(h))
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
