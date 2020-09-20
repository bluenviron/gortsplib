package gortsplib

// MultiBuffer implements software multi buffering, that allows to reuse
// existing buffers without creating new ones, increasing performance.
type MultiBuffer struct {
	count   int
	buffers [][]byte
	cur     int
}

// NewMultiBuffer allocates a MultiBuffer.
func NewMultiBuffer(count int, size int) *MultiBuffer {
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
	mb.cur += 1
	if mb.cur >= mb.count {
		mb.cur = 0
	}
	return ret
}

type multiFrame struct {
	count  int
	frames []*InterleavedFrame
	cur    int
}

func newMultiFrame(count int, bufsize int) *multiFrame {
	frames := make([]*InterleavedFrame, count)
	for i := 0; i < count; i++ {
		frames[i] = &InterleavedFrame{
			Content: make([]byte, 0, bufsize),
		}
	}

	return &multiFrame{
		count:  count,
		frames: frames,
	}
}

func (mf *multiFrame) next() *InterleavedFrame {
	ret := mf.frames[mf.cur]
	mf.cur += 1
	if mf.cur >= mf.count {
		mf.cur = 0
	}

	ret.Content = ret.Content[:cap(ret.Content)]

	return ret
}
