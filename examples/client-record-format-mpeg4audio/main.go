//go:build cgo

package main

import (
	"crypto/rand"
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
)

// This example shows how to
// 1. connect to a RTSP server, announce a MPEG-4 Audio (AAC) format
// 2. generate dummy LPCM audio samples
// 3. encode audio samples with MPEG-4 Audio (AAC)
// 3. generate RTP packets from MPEG-4 Audio units
// 4. write RTP packets to the server

// This example requires the FFmpeg libraries, that can be installed with this command:
// apt install -y libavcodec-dev gcc pkg-config

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
	// create a description that contains a MPEG-4 Audio format
	forma := &format.MPEG4Audio{
		PayloadTyp: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 1,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
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

	// setup LPCM -> MPEG-4 Audio encoder
	mp4aEnc := &mp4aEncoder{}
	err = mp4aEnc.initialize()
	if err != nil {
		panic(err)
	}

	// setup MPEG-4 Audio -> RTP encoder
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

		// encode samples with MPEG-4 Audio
		aus, outPTS, err := mp4aEnc.encode(samples)
		if err != nil {
			panic(err)
		}
		if aus == nil {
			continue
		}

		// generate RTP packets from MPEG-4 audio access units
		pkts, err := rtpEnc.Encode(aus)
		if err != nil {
			panic(err)
		}

		log.Printf("writing RTP packets with PTS=%d, packet count=%d", outPTS, len(pkts))

		for _, pkt := range pkts {
			pkt.Timestamp += uint32(int64(randomStart) + outPTS)

			err = c.WritePacketRTP(desc.Medias[0], pkt)
			if err != nil {
				panic(err)
			}
		}

		prevPTS = pts
	}
}
