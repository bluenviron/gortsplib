// Package multibuffer contains a buffer with multiple levels.
package multibuffer

// MultiBuffer implements software multi buffering, that allows to reuse
// existing buffers without creating new ones, improving performance.
type MultiBuffer struct {
	count   uint64
	buffers [][]byte
	cur     uint64
}

// New allocates a MultiBuffer.
func New(count uint64, size uint64) *MultiBuffer {
	buffers := make([][]byte, count)
	for i := uint64(0); i < count; i++ {
		buffers[i] = make([]byte, size)
	}

	return &MultiBuffer{
		count:   count,
		buffers: buffers,
	}
}

// Next gets the current buffer and sets the next buffer as the current one.
func (mb *MultiBuffer) Next() []byte {
	ret := mb.buffers[mb.cur%mb.count]
	mb.cur++
	return ret
}
