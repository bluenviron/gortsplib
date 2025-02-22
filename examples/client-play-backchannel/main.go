package main

import (
	"crypto/rand"
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/g711"
)

// This example shows how to
// 1. generate a dummy G711 audio stream
// 2. connect to a RTSP server, find a back channel that supports G711
// 3. route the G711 stream to the channel

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

func findG711BackChannel(desc *description.Session) (*description.Media, *format.G711) {
	for _, media := range desc.Medias {
		if media.IsBackChannel {
			for _, forma := range media.Formats {
				if g711, ok := forma.(*format.G711); ok {
					return media, g711
				}
			}
		}
	}
	return nil, nil
}

func main() {
	c := gortsplib.Client{
		RequestBackChannels: true,
	}

	// parse URL
	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// find published medias
	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the back channel
	medi, forma := findG711BackChannel(desc)
	if medi == nil {
		panic("media not found")
	}

	// setup a single media
	_, err = c.Setup(desc.BaseURL, medi, 0, 0)
	if err != nil {
		panic(err)
	}

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

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
		if forma.MULaw {
			samples, err = g711.Mulaw(samples).Marshal()
			if err != nil {
				panic(err)
			}
		} else {
			samples, err = g711.Alaw(samples).Marshal()
			if err != nil {
				panic(err)
			}
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

			err = c.WritePacketRTP(desc.Medias[0], pkt)
			if err != nil {
				panic(err)
			}
		}

		prevPTS = pts
	}
}
