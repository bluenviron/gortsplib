// Package asyncprocessor contains an asynchronous processor.
package asyncprocessor

import (
	"context"

	"github.com/bluenviron/gortsplib/v5/pkg/ringbuffer"
)

// Processor is an asynchronous queue processor
// that allows to detach the routine that is reading a stream
// from the routine that is writing a stream.
type Processor struct {
	BufferSize int
	OnError    func(context.Context, error)

	running   bool
	buffer    *ringbuffer.RingBuffer
	ctx       context.Context
	ctxCancel func()

	done chan struct{}
}

// Initialize initializes the processor.
func (w *Processor) Initialize() {
	w.buffer, _ = ringbuffer.New(uint64(w.BufferSize))
	w.ctx, w.ctxCancel = context.WithCancel(context.Background())
	w.done = make(chan struct{})
}

// Close closes the processor.
func (w *Processor) Close() {
	w.ctxCancel()
	w.buffer.Close()

	if w.running {
		<-w.done
	}
}

// Start starts the processor.
func (w *Processor) Start() {
	w.running = true
	go w.run()
}

func (w *Processor) run() {
	defer close(w.done)

	err := w.runInner()
	w.OnError(w.ctx, err)
}

func (w *Processor) runInner() error {
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

// Push pushes data to the queue.
func (w *Processor) Push(cb func() error) bool {
	return w.buffer.Push(cb)
}
