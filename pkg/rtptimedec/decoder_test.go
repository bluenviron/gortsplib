package rtptimedec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOverflow(t *testing.T) {
	d := New(90000)

	i := uint32(4294877296)
	pts := d.Decode(i)
	require.Equal(t, time.Duration(0), pts)

	// 1st overflow
	i += 90000 * 2
	pts = d.Decode(i)
	require.Equal(t, 2*time.Second, pts)

	// reach 4294890000 slowly
	for ; i < 4294890000; i += 90000 * 10 {
		pts = d.Decode(i)
		require.Equal(t, 2*time.Second+time.Second*time.Duration(i-90000)/90000, pts)
	}

	// 2nd overflow
	i += 90000 * 10
	pts = d.Decode(i)
	require.Equal(t, 47732*time.Second, pts)
}

func TestOverflowAndBack(t *testing.T) {
	d := New(90000)

	pts := d.Decode(4294877296)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(90000)
	require.Equal(t, 2*time.Second, pts)

	pts = d.Decode(4294877296)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(4294877296 - 90000)
	require.Equal(t, -1*time.Second, pts)

	pts = d.Decode(4294877296)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(90000)
	require.Equal(t, 2*time.Second, pts)
}
