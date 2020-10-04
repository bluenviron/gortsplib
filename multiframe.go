package gortsplib

import (
	"github.com/aler9/gortsplib/base"
)

type multiFrame struct {
	count  int
	frames []*base.InterleavedFrame
	cur    int
}

func newMultiFrame(count int, bufsize int) *multiFrame {
	frames := make([]*base.InterleavedFrame, count)
	for i := 0; i < count; i++ {
		frames[i] = &base.InterleavedFrame{
			Content: make([]byte, 0, bufsize),
		}
	}

	return &multiFrame{
		count:  count,
		frames: frames,
	}
}

func (mf *multiFrame) next() *base.InterleavedFrame {
	ret := mf.frames[mf.cur]
	mf.cur += 1
	if mf.cur >= mf.count {
		mf.cur = 0
	}

	ret.Content = ret.Content[:cap(ret.Content)]

	return ret
}
