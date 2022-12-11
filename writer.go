package gortsplib

import (
	"github.com/aler9/gortsplib/v2/pkg/ringbuffer"
)

// this struct contains a queue that allows to detach the routine that is reading a stream
// from the routine that is writing a stream.
type writer struct {
	running bool
	buffer  *ringbuffer.RingBuffer

	done chan struct{}
}

func (w *writer) allocateBuffer(size int) {
	w.buffer, _ = ringbuffer.New(uint64(size))
}

func (w *writer) start() {
	w.running = true
	w.done = make(chan struct{})
	go w.run()
}

func (w *writer) stop() {
	if w.running {
		w.buffer.Close()
		<-w.done
		w.running = false
	}
}

func (w *writer) run() {
	defer close(w.done)

	for {
		tmp, ok := w.buffer.Pull()
		if !ok {
			return
		}

		tmp.(func())()
	}
}

func (w *writer) queue(cb func()) {
	w.buffer.Push(cb)
}
