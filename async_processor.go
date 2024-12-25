package gortsplib

import (
	"github.com/bluenviron/gortsplib/v4/pkg/ringbuffer"
)

// this is an asynchronous queue processor
// that allows to detach the routine that is reading a stream
// from the routine that is writing a stream.
type asyncProcessor struct {
	bufferSize int

	running bool
	buffer  *ringbuffer.RingBuffer

	chError chan error
}

func (w *asyncProcessor) initialize() {
	w.buffer, _ = ringbuffer.New(uint64(w.bufferSize))
}

func (w *asyncProcessor) start() {
	w.running = true
	w.chError = make(chan error)
	go w.run()
}

func (w *asyncProcessor) stop() {
	if w.running {
		w.buffer.Close()
		<-w.chError
		w.running = false
	}
}

func (w *asyncProcessor) run() {
	err := w.runInner()
	w.chError <- err
	close(w.chError)
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
