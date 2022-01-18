package rtptimedec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOverflow(t *testing.T) {
	d := New(90000)
	var pts time.Duration

	for _, ts := range []uint32{
		4294877296,
		90001,
		3240090001,
		565122706,
	} {
		pts = d.Decode(ts)
	}

	require.Equal(t, 15*60*60*time.Second+2*time.Second, pts)
}
