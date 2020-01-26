package gortsplib

import (
	"bufio"
	"fmt"
	"sort"
)

const (
	_MAX_HEADER_COUNT        = 255
	_MAX_HEADER_KEY_LENGTH   = 255
	_MAX_HEADER_VALUE_LENGTH = 255
)

type Header map[string][]string

func readHeader(rb *bufio.Reader) (Header, error) {
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

		err = readByteEqual(rb, ' ')
		if err != nil {
			return nil, err
		}

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
