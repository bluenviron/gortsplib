// Package multibuffer implements a buffer with multiple levels.
package multibuffer

// MultiBuffer implements software multi buffering, that allows to reuse
// existing buffers without creating new ones, increasing performance.
type MultiBuffer struct {
	count   int
	buffers [][]byte
	cur     int
}

// New allocates a MultiBuffer.
func New(count int, size int) *MultiBuffer {
	buffers := make([][]byte, count)
	for i := 0; i < count; i++ {
		buffers[i] = make([]byte, size)
	}

	return &MultiBuffer{
		count:   count,
		buffers: buffers,
	}
}

// Next gets the current buffer and sets the next buffer as the current one.
func (mb *MultiBuffer) Next() []byte {
	ret := mb.buffers[mb.cur]
	mb.cur++
	if mb.cur >= mb.count {
		mb.cur = 0
	}
	return ret
}
