package gortsplib

import (
	"sync"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type clientReader struct {
	c *Client

	mutex                  sync.Mutex
	allowInterleavedFrames bool
}

func (r *clientReader) start() {
	go r.run()
}

func (r *clientReader) setAllowInterleavedFrames(v bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.allowInterleavedFrames = v
}

func (r *clientReader) wait() {
	for {
		select {
		case <-r.c.chResponse:
		case <-r.c.chRequest:
		case <-r.c.chReadError:
			return
		}
	}
}

func (r *clientReader) run() {
	r.c.chReadError <- r.runInner()
}

func (r *clientReader) runInner() error {
	for {
		what, err := r.c.conn.Read()
		if err != nil {
			return err
		}

		switch what := what.(type) {
		case *base.Response:
			r.c.chResponse <- what

		case *base.Request:
			r.c.chRequest <- what

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
