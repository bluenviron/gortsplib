// Package base64streamreader contains a base64 reader for a stream-based connection.
package base64streamreader

import (
	"bytes"
	"encoding/base64"
	"io"
)

const (
	readSize = 1024
)

type reader struct {
	r       io.Reader
	predec  []byte
	postdec []byte
}

func (r *reader) Read(p []byte) (int, error) {
	for len(r.postdec) == 0 {
		todec := r.predec

		if len(todec)%4 != 0 {
			todec = todec[:(len(todec)/4)*4]
		}

		if i := bytes.IndexByte(todec, '='); i >= 0 {
			if len(todec) > (i+1) && todec[i+1] == '=' {
				i++
			}
			todec = todec[:i+1]
		}

		if len(todec) == 0 {
			buf := make([]byte, readSize)
			n, err := r.r.Read(buf)
			if err != nil && n == 0 {
				return 0, err
			}

			r.predec = append(r.predec, buf[:n]...)
			continue
		}

		r.predec = r.predec[len(todec):]

		out, err := base64.StdEncoding.DecodeString(string(todec))
		if err != nil {
			return 0, err
		}

		r.postdec = append(r.postdec, out...)
	}

	n := copy(p, r.postdec)
	r.postdec = r.postdec[n:]

	return n, nil
}

// New allocates a base64 stream reader.
func New(r io.Reader) io.Reader {
	return &reader{r: r}
}
