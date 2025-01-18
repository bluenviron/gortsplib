package gortsplib

import (
	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
)

// this is an asynchronous queue processor
// that allows to detach the routine that is reading a stream
// from the routine that is writing a stream.
type asyncProcessor struct {
	bufferSize int

	running   bool
	buffer    *ringbuffer.RingBuffer
	stopError error

	stopped chan struct{}
}

func (w *asyncProcessor) initialize() {
	w.buffer, _ = ringbuffer.New(uint64(w.bufferSize))
}

func (w *asyncProcessor) start() {
	w.running = true
	w.stopped = make(chan struct{})
	go w.run()
}

func (w *asyncProcessor) stop() {
	if w.running {
		w.buffer.Close()
		<-w.stopped
		w.running = false
	}
}

func (w *asyncProcessor) run() {
	w.stopError = w.runInner()
	close(w.stopped)
}

func (w *asyncProcessor) runInner() error {
	for {
		tmp, ok := w.buffer.Pull()
		if !ok {
			return nil
		}

		err := tmp.(func() error)()
		if err != nil {
			return err
		}
	}
}

func (w *asyncProcessor) push(cb func() error) bool {
	return w.buffer.Push(cb)
}
