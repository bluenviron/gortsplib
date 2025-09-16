// Package rtptime contains a time decoder.
package rtptime

import (
	"sync"
	"time"

	"github.com/pion/rtp"
)

var timeNow = time.Now

// avoid an int64 overflow and preserve resolution by splitting division into two parts:
// first add the integer part, then the decimal part.
func multiplyAndDivide(v, m, d int64) int64 {
	secs := v / d
	dec := v % d
	return (secs*m + dec*m/d)
}

type globalDecoderTrackData struct {
	overall int64
	prev    uint32
}

func (d *globalDecoderTrackData) decode(ts uint32) int64 {
	d.overall += int64(int32(ts - d.prev))
	d.prev = ts
	return d.overall
}

// GlobalDecoderTrack is a track (RTSP format or WebRTC track) of GlobalDecoder.
type GlobalDecoderTrack interface {
	ClockRate() int
	PTSEqualsDTS(*rtp.Packet) bool
}

// GlobalDecoder is a RTP timestamp decoder.
type GlobalDecoder struct {
	mutex             sync.Mutex
	leadingTrack      GlobalDecoderTrack
	startNTP          time.Time
	startPTS          int64
	startPTSClockRate int64
	tracks            map[GlobalDecoderTrack]*globalDecoderTrackData
}

// Initialize initializes a GlobalDecoder.
func (d *GlobalDecoder) Initialize() {
	d.tracks = make(map[GlobalDecoderTrack]*globalDecoderTrackData)
}

// Decode decodes a timestamp.
func (d *GlobalDecoder) Decode(
	track GlobalDecoderTrack,
	pkt *rtp.Packet,
) (int64, bool) {
	if track.ClockRate() == 0 {
		return 0, false
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	df, ok := d.tracks[track]

	// never seen before track
	if !ok {
		if !track.PTSEqualsDTS(pkt) {
			return 0, false
		}

		now := timeNow()

		if d.leadingTrack == nil {
			d.leadingTrack = track
			d.startNTP = now
			d.startPTS = 0
			d.startPTSClockRate = int64(track.ClockRate())
		}

		// start from the PTS of the leading track
		startPTS := multiplyAndDivide(d.startPTS, int64(track.ClockRate()), d.startPTSClockRate)
		startPTS += multiplyAndDivide(int64(now.Sub(d.startNTP)), int64(track.ClockRate()), int64(time.Second))

		d.tracks[track] = &globalDecoderTrackData{
			overall: startPTS,
			prev:    pkt.Timestamp,
		}

		return startPTS, true
	}

	pts := df.decode(pkt.Timestamp)

	// update startNTP / startPTS
	if d.leadingTrack == track && track.PTSEqualsDTS(pkt) {
		now := timeNow()
		d.startNTP = now
		d.startPTS = pts
	}

	return pts, true
}
