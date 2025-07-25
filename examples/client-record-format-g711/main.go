// Package main contains an example.
package main

import (
	"crypto/rand"
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/g711"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server, announce a G711 format.
// 2. generate dummy LPCM audio samples.
// 3. encode audio samples with G711.
// 3. generate RTP packets from G711 samples.
// 4. write RTP packets to the server.

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

func main() {
	// create a description that contains a G711 format
	forma := &format.G711{
		PayloadTyp:   0,
		MULaw:        true,
		SampleRate:   8000,
		ChannelCount: 1,
	}
	desc := &description.Session{
		Medias: []*description.Media{{
			Type:    description.MediaTypeAudio,
			Formats: []format.Format{forma},
		}},
	}

	// connect to the server and start recording
	c := gortsplib.Client{}
	err := c.StartRecording("rtsp://myuser:mypass@localhost:8554/mystream", desc)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// setup G711 -> RTP encoder
	rtpEnc, err := forma.CreateEncoder()
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
		pts := multiplyAndDivide(int64(time.Since(start)), int64(forma.ClockRate()), int64(time.Second))

		// generate dummy LPCM audio samples
		samples := createDummyAudio(pts, prevPTS)

		// encode samples with G711
		samples, err = g711.Mulaw(samples).Marshal()
		if err != nil {
			panic(err)
		}

		// generate RTP packets from G711 samples
		var pkts []*rtp.Packet
		pkts, err = rtpEnc.Encode(samples)
		if err != nil {
			panic(err)
		}

		log.Printf("writing RTP packets with PTS=%d, sample size=%d, pkt count=%d", prevPTS, len(samples), len(pkts))

		// write RTP packets to the server
		for _, pkt := range pkts {
			pkt.Timestamp += uint32(int64(randomStart) + prevPTS)

			err = c.WritePacketRTP(desc.Medias[0], pkt)
			if err != nil {
				panic(err)
			}
		}

		prevPTS = pts
	}
}
