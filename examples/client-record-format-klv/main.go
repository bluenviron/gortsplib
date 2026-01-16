// Package main contains an example.
package main

import (
	"crypto/rand"
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

// This example shows how to:
// 1. connect to a RTSP server, announce a KLV format.
// 2. generate dummy KLV units.
// 4. generate RTP packets from KLV.
// 5. write RTP packets to the server.

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
	// create a stream description that contains a KLV format
	forma := &format.KLV{
		PayloadTyp: 96,
	}
	desc := &description.Session{
		Medias: []*description.Media{{
			Type:    description.MediaTypeApplication,
			Formats: []format.Format{forma},
		}},
	}

	// connect to the server, announce the format and start recording
	c := gortsplib.Client{}
	err := c.StartRecording("rtsp://myuser:mypass@localhost:8554/mystream", desc)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// setup KLV -> RTP encoder
	rtpEnc, err := forma.CreateEncoder()
	if err != nil {
		panic(err)
	}

	start := time.Now()

	randomStart, err := randUint32()
	if err != nil {
		panic(err)
	}

	// setup a ticker to sleep between frames
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		// get current timestamp
		pts := multiplyAndDivide(int64(time.Since(start)), int64(forma.ClockRate()), int64(time.Second))

		buf := []byte{
			0x06, 0x0e, 0x2b, 0x34, // universal label key
			0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01,
			0x03,             // length
			0x01, 0x02, 0x03, // value
		}

		// generate RTP packets from the VP9 frame
		var pkts []*rtp.Packet
		pkts, err = rtpEnc.Encode(buf)
		if err != nil {
			panic(err)
		}

		log.Printf("writing RTP packets with PTS=%d, frame size=%d, pkt count=%d", pts, len(buf), len(pkts))

		// write RTP packets to the server
		for _, pkt := range pkts {
			pkt.Timestamp += uint32(int64(randomStart) + pts)

			err = c.WritePacketRTP(desc.Medias[0], pkt)
			if err != nil {
				panic(err)
			}
		}
	}
}
