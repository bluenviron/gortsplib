package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/g711"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
)

func multiplyAndDivide(v, m, d int64) int64 {
	secs := v / d
	dec := v % d
	return (secs*m + dec*m/d)
}

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func findTrack(r *mpegts.Reader) (*mpegts.Track, error) {
	for _, track := range r.Tracks() {
		if _, ok := track.Codec.(*mpegts.CodecH264); ok {
			return track, nil
		}
	}
	return nil, fmt.Errorf("H264 track not found")
}

type audioStreamer struct {
	stream *gortsplib.ServerStream
}

func (r *audioStreamer) initialize() {
	go r.run()
}

func (r *audioStreamer) close() {
}

func (r *audioStreamer) run() {
	// setup G711 -> RTP encoder
	rtpEnc, err := r.stream.Desc.Medias[0].Formats[0].(*format.G711).CreateEncoder()
	if err != nil {
		panic(err)
	}

	start := time.Now()
	prevPTS := int64(0)

	randomStart, err := randUint32()
	if err != nil {
		panic(err)
	}

	// setup a ticker to sleep between writings
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// get current timestamp
		pts := multiplyAndDivide(int64(time.Since(start)), int64(r.stream.Desc.Medias[0].Formats[0].ClockRate()), int64(time.Second))

		// generate dummy LPCM audio samples
		samples := createDummyAudio(pts, prevPTS)

		// encode samples with G711
		samples, err = g711.Mulaw(samples).Marshal()
		if err != nil {
			panic(err)
		}

		// generate RTP packets from G711 samples
		pkts, err := rtpEnc.Encode(samples)
		if err != nil {
			panic(err)
		}

		log.Printf("writing RTP packets with PTS=%d, sample size=%d, pkt count=%d", prevPTS, len(samples), len(pkts))

		// write RTP packets to the server
		for _, pkt := range pkts {
			pkt.Timestamp += uint32(int64(randomStart) + prevPTS)

			err = r.stream.WritePacketRTP(r.stream.Desc.Medias[0], pkt)
			if err != nil {
				panic(err)
			}
		}

		prevPTS = pts

	}
}
