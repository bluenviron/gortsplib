package gortsplib

import (
	"github.com/aler9/gortsplib/v2/pkg/ringbuffer"
)

// this struct contains a queue that allows to detach the routine that is reading a stream
// from the routine that is writing a stream.
type clientWriter struct {
	allowWriting bool
	buffer       *ringbuffer.RingBuffer

	done chan struct{}
}

func (cw *clientWriter) start(c *Client) {
	if c.state == clientStatePlay {
		// when reading, buffer is only used to send RTCP receiver reports,
		// that are much smaller than RTP packets and are sent at a fixed interval.
		// decrease RAM consumption by allocating less buffers.
		cw.buffer, _ = ringbuffer.New(8)
	} else {
		cw.buffer, _ = ringbuffer.New(uint64(c.WriteBufferCount))
	}

	cw.done = make(chan struct{})
	go cw.run()

	cw.allowWriting = true
}

func (cw *clientWriter) stop() {
	cw.allowWriting = false

	cw.buffer.Close()
	<-cw.done
	cw.buffer = nil
}

func (cw *clientWriter) run() {
	defer close(cw.done)

	for {
		tmp, ok := cw.buffer.Pull()
		if !ok {
			return
		}

		tmp.(func())()
	}
}

func (cw *clientWriter) queue(cb func()) bool {
	if !cw.allowWriting {
		return false
	}

	cw.buffer.Push(cb)
	return true
}
