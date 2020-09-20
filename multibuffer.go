package gortsplib

// MultiBuffer implements software multi buffering, that allows to reuse
// existing buffers without creating new ones, increasing performance.
type MultiBuffer struct {
	buffers [][]byte
	curBuf  int
}

// NewMultiBuffer allocates a MultiBuffer.
func NewMultiBuffer(count int, size int) *MultiBuffer {
	buffers := make([][]byte, count)
	for i := 0; i < count; i++ {
		buffers[i] = make([]byte, size)
	}

	return &MultiBuffer{
		buffers: buffers,
	}
}

// Next gets the current buffer and sets the next buffer as the current one.
func (mb *MultiBuffer) Next() []byte {
	ret := mb.buffers[mb.curBuf]
	mb.curBuf += 1
	if mb.curBuf >= len(mb.buffers) {
		mb.curBuf = 0
	}
	return ret
}
