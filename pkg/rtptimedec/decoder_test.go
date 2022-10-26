package rtptimedec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNegativeDiff(t *testing.T) {
	d := New(90000)

	i := uint32(0)
	pts := d.Decode(i)
	require.Equal(t, time.Duration(0), pts)

	i += 90000 * 2
	pts = d.Decode(i)
	require.Equal(t, 2*time.Second, pts)

	i -= 90000 * 1
	pts = d.Decode(i)
	require.Equal(t, 1*time.Second, pts)

	i += 90000 * 2
	pts = d.Decode(i)
	require.Equal(t, 3*time.Second, pts)
}

func TestOverflow(t *testing.T) {
	d := New(90000)

	i := uint32(0xFFFFFFFF - 90000 + 1)
	secs := time.Duration(0)
	pts := d.Decode(i)
	require.Equal(t, time.Duration(0), pts)

	const stride = 1500
	lim := uint32(uint64(0xFFFFFFFF + 1 - (stride * 90000)))

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

	pts := d.Decode(0xFFFFFFFF - 90000 + 1)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(90000)
	require.Equal(t, 2*time.Second, pts)

	pts = d.Decode(0xFFFFFFFF - 90000 + 1)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(0xFFFFFFFF - 90000 + 1 - 90000)
	require.Equal(t, -1*time.Second, pts)

	pts = d.Decode(0xFFFFFFFF - 90000 + 1)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(90000)
	require.Equal(t, 2*time.Second, pts)
}

func BenchmarkDecoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		func() {
			d := New(90000)
			n := uint32(0)
			for j := 0; j < 200; j++ {
				if (j % 2) == 0 {
					n += 90000
				} else {
					n -= 45000
				}
				d.Decode(n)
			}
		}()
	}
}
