//go:build cgo

package main

import "math"

const (
	sampleRate = 48000
	frequency  = 400
	amplitude  = (1 << 14) - 1
)

func createDummyAudio(pts int64, prevPTS int64) []byte {
	sampleCount := (pts - prevPTS)
	n := 0
	ret := make([]byte, sampleCount*2)

	for i := int64(0); i < sampleCount; i++ {
		v := int16(amplitude * math.Sin((float64(prevPTS+i)*frequency*math.Pi*2)/sampleRate))
		ret[n] = byte(v >> 8)
		ret[n+1] = byte(v)
		n += 2
	}

	return ret
}
