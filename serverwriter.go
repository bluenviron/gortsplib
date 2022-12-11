package gortsplib

import (
	"github.com/aler9/gortsplib/v2/pkg/ringbuffer"
)

type serverWriter struct {
	running bool
	buffer  *ringbuffer.RingBuffer

	done chan struct{}
}

func (sw *serverWriter) start() {
	if !sw.running {
		sw.running = true
		sw.done = make(chan struct{})
		go sw.run()
	}
}

func (sw *serverWriter) stop() {
	if sw.running {
		sw.buffer.Close()
		<-sw.done
		sw.running = false
	}
}

func (sw *serverWriter) run() {
	defer close(sw.done)

	for {
		tmp, ok := sw.buffer.Pull()
		if !ok {
			return
		}

		tmp.(func())()
	}
}

func (sw *serverWriter) queue(cb func()) {
	sw.buffer.Push(cb)
}
