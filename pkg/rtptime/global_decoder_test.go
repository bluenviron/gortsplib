package rtptime

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecoderNegativeDiff(t *testing.T) {
	i := uint32(0)
	d := newGlobalDecoderTrackData(0, 90000, i)

	i += 90000 * 2
	pts := d.decode(i)
	require.Equal(t, 2*time.Second, pts)

	i -= 90000 * 1
	pts = d.decode(i)
	require.Equal(t, 1*time.Second, pts)

	i += 90000 * 2
	pts = d.decode(i)
	require.Equal(t, 3*time.Second, pts)
}

func TestDecoderOverflow(t *testing.T) {
	secs := time.Duration(0)
	i := uint32(0xFFFFFFFF - 90000 + 1)
	d := newGlobalDecoderTrackData(0, 90000, i)

	const stride = 1500
	lim := uint32(uint64(0xFFFFFFFF + 1 - (stride * 90000)))

	for n := 0; n < 100; n++ {
		// overflow
		i += 90000 * stride
		secs += stride
		pts := d.decode(i)
		require.Equal(t, secs*time.Second, pts)

		// reach 2^32 slowly
		secs += stride
		i += 90000 * stride
		for ; i < lim; i += 90000 * stride {
			pts = d.decode(i)
			require.Equal(t, secs*time.Second, pts)
			secs += stride
		}
	}
}

func TestDecoderOverflowAndBack(t *testing.T) {
	d := newGlobalDecoderTrackData(0, 90000, 0xFFFFFFFF-90000+1)

	pts := d.decode(90000)
	require.Equal(t, 2*time.Second, pts)

	pts = d.decode(0xFFFFFFFF - 90000 + 1)
	require.Equal(t, time.Duration(0), pts)

	pts = d.decode(0xFFFFFFFF - 90000 + 1 - 90000)
	require.Equal(t, -1*time.Second, pts)

	pts = d.decode(0xFFFFFFFF - 90000 + 1)
	require.Equal(t, time.Duration(0), pts)

	pts = d.decode(90000)
	require.Equal(t, 2*time.Second, pts)
}

func BenchmarkDecoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		func() {
			n := uint32(0)
			d := newGlobalDecoderTrackData(0, 90000, n)
			for j := 0; j < 200; j++ {
				if (j % 2) == 0 {
					n += 90000
				} else {
					n -= 45000
				}
				d.decode(n)
			}
		}()
	}
}

type dummyTrack struct {
	clockRate    int
	ptsEqualsDTS bool
}

func (t *dummyTrack) ClockRate() int {
	return t.clockRate
}

func (t *dummyTrack) PTSEqualsDTS(*rtp.Packet) bool {
	return t.ptsEqualsDTS
}

func TestGlobalDecoder(t *testing.T) {
	g := NewGlobalDecoder()

	t1 := &dummyTrack{clockRate: 90000}
	t2 := &dummyTrack{clockRate: 48000, ptsEqualsDTS: true}

	timeNow = func() time.Time {
		return time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	}

	_, ok := g.Decode(t1, &rtp.Packet{})
	require.Equal(t, false, ok)

	t1.ptsEqualsDTS = true
	pts, ok := g.Decode(t1, &rtp.Packet{Header: rtp.Header{Timestamp: 22500}})
	require.Equal(t, true, ok)
	require.Equal(t, time.Duration(0), pts)

	timeNow = func() time.Time {
		return time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	}

	pts, ok = g.Decode(t1, &rtp.Packet{Header: rtp.Header{Timestamp: 22500 + 90000}})
	require.Equal(t, true, ok)
	require.Equal(t, 1*time.Second, pts)

	timeNow = func() time.Time {
		return time.Date(2008, 0o5, 20, 22, 15, 25, 0, time.UTC)
	}

	pts, ok = g.Decode(t2, &rtp.Packet{Header: rtp.Header{Timestamp: 33100}})
	require.Equal(t, true, ok)
	require.Equal(t, 5*time.Second, pts)

	pts, ok = g.Decode(t2, &rtp.Packet{Header: rtp.Header{Timestamp: 33100 + 48000}})
	require.Equal(t, true, ok)
	require.Equal(t, 6*time.Second, pts)
}

func TestGlobalDecoderInvalidClockRate(t *testing.T) {
	g := NewGlobalDecoder()

	tr := &dummyTrack{clockRate: 0, ptsEqualsDTS: true}

	_, ok := g.Decode(tr, &rtp.Packet{Header: rtp.Header{Timestamp: 90000}})
	require.Equal(t, false, ok)
}
