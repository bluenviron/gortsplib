package gortsplib

import (
	"sync"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type clientReader struct {
	c                      *Client
	mutex                  sync.Mutex
	allowInterleavedFrames bool
}

func newClientReader(c *Client) *clientReader {
	r := &clientReader{
		c: c,
	}

	go r.run()

	return r
}

func (r *clientReader) setAllowInterleavedFrames(v bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.allowInterleavedFrames = v
}

func (r *clientReader) wait() {
	for {
		select {
		case <-r.c.chReadError:
			return

		case <-r.c.chReadResponse:
		case <-r.c.chReadRequest:
		}
	}
}

func (r *clientReader) run() {
	err := r.runInner()
	r.c.readError(err)
}

func (r *clientReader) runInner() error {
	for {
		what, err := r.c.conn.Read()
		if err != nil {
			return err
		}

		switch what := what.(type) {
		case *base.Response:
			r.c.readResponse(what)

		case *base.Request:
			r.c.readRequest(what)

		case *base.InterleavedFrame:
			r.mutex.Lock()

			if !r.allowInterleavedFrames {
				r.mutex.Unlock()
				return liberrors.ErrClientUnexpectedFrame{}
			}

			if cb, ok := r.c.tcpCallbackByChannel[what.Channel]; ok {
				cb(what.Payload)
			}
			r.mutex.Unlock()
		}
	}
}
