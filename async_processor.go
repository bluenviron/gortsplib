package gortsplib

import (
	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
)

// this struct contains a queue that allows to detach the routine that is reading a stream
// from the routine that is writing a stream.
type asyncProcessor struct {
	running bool
	buffer  *ringbuffer.RingBuffer

	done chan struct{}
}

func (w *asyncProcessor) allocateBuffer(size int) {
	w.buffer, _ = ringbuffer.New(uint64(size))
}

func (w *asyncProcessor) start() {
	w.running = true
	w.done = make(chan struct{})
	go w.run()
}

func (w *asyncProcessor) stop() {
	if w.running {
		w.buffer.Close()
		<-w.done
		w.running = false
	}
}

func (w *asyncProcessor) run() {
	defer close(w.done)

	for {
		tmp, ok := w.buffer.Pull()
		if !ok {
			return
		}

		tmp.(func())()
	}
}

func (w *asyncProcessor) push(cb func()) bool {
	return w.buffer.Push(cb)
}
