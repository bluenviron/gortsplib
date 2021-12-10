package h264

import (
	"time"
)

// DTSEstimator is a DTS estimator.
type DTSEstimator struct {
	initializing int
	prevDTS      time.Duration
	prevPTS      time.Duration
}

// NewDTSEstimator allocates a DTSEstimator.
func NewDTSEstimator() *DTSEstimator {
	return &DTSEstimator{
		initializing: 2,
	}
}

// Feed provides PTS to the estimator, and returns the estimated DTS.
func (d *DTSEstimator) Feed(pts time.Duration) time.Duration {
	switch d.initializing {
	case 2:
		d.initializing--
		return 0

	case 1:
		d.initializing--
		d.prevPTS = pts
		d.prevDTS = time.Millisecond
		return time.Millisecond
	}

	dts := func() time.Duration {
		// PTS is increasing
		// use previous PTS
		if pts > d.prevPTS {
			return d.prevPTS
		}

		// PTS is not increasing
		// use last DTS value plus a small quantity
		return d.prevDTS + time.Millisecond
	}()

	d.prevPTS = pts
	d.prevDTS = dts

	return dts
}
