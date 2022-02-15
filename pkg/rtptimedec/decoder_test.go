package rtptimedec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOverflow(t *testing.T) {
	d := New(90000)

	i := uint32(4294877296)
	secs := time.Duration(0)
	pts := d.Decode(i)
	require.Equal(t, time.Duration(0), pts)

	const stride = 150
	lim := uint32(uint64(4294967296 - (stride * 90000)))

	for n := 0; n < 100; n++ {
		// overflow
		i += 90000 * stride
		secs += stride
		pts = d.Decode(i)
		require.Equal(t, secs*time.Second, pts)

		// reach 2^32 slowly
		secs += stride
		i += 90000 * stride
		for ; i < lim; i += 90000 * stride {
			pts = d.Decode(i)
			require.Equal(t, secs*time.Second, pts)
			secs += stride
		}
	}
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
