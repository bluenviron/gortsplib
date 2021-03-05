package ringbuffer

import (
	"sync"
)

type event struct {
	mutex sync.Mutex
	cond  *sync.Cond
	value bool
}

func newEvent() *event {
	cv := &event{}
	cv.cond = sync.NewCond(&cv.mutex)
	return cv
}

func (cv *event) signal() {
	func() {
		cv.mutex.Lock()
		defer cv.mutex.Unlock()
		cv.value = true
	}()

	cv.cond.Broadcast()
}

func (cv *event) wait() {
	cv.mutex.Lock()
	defer cv.mutex.Unlock()

	if !cv.value {
		cv.cond.Wait()
	}

	cv.value = false
}
