package gortsplib

import (
	"sync/atomic"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type clientReader struct {
	c                      *Client
	allowInterleavedFrames atomic.Bool
}

func newClientReader(c *Client) *clientReader {
	r := &clientReader{
		c: c,
	}

	go r.run()

	return r
}

func (r *clientReader) setAllowInterleavedFrames(v bool) {
	r.allowInterleavedFrames.Store(v)
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
			if !r.allowInterleavedFrames.Load() {
				return liberrors.ErrClientUnexpectedFrame{}
			}

			if cb, ok := r.c.tcpCallbackByChannel[what.Channel]; ok {
				cb(what.Payload)
			}
		}
	}
}
