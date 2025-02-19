//go:build cgo

package main

import (
	"crypto/rand"
	"image"
	"image/color"
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
)

// This example shows how to
// 1. connect to a RTSP server, announce a VP8 format
// 2. generate dummy RGBA images
// 3. encode images with VP8
// 4. generate RTP packets from VP8
// 5. write RTP packets to the server

// This example requires the FFmpeg libraries, that can be installed with this command:
// apt install -y libavformat-dev libswscale-dev gcc pkg-config

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

func createDummyImage(i int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))

	var cl color.RGBA
	switch i {
	case 0:
		cl = color.RGBA{255, 0, 0, 0}
	case 1:
		cl = color.RGBA{0, 255, 0, 0}
	case 2:
		cl = color.RGBA{0, 0, 255, 0}
	}

	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			img.SetRGBA(x, y, cl)
		}
	}

	return img
}

func main() {
	// create a stream description that contains a VP8 format
	forma := &format.VP8{
		PayloadTyp: 96,
	}
	desc := &description.Session{
		Medias: []*description.Media{{
			Type:    description.MediaTypeVideo,
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

	// setup RGBA -> VP8 encoder
	vp8enc := &vp8Encoder{
		Width:  640,
		Height: 480,
		FPS:    5,
	}
	err = vp8enc.initialize()
	if err != nil {
		panic(err)
	}
	defer vp8enc.close()

	// setup VP8 -> RTP encoder
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

	i := 0

	for range ticker.C {
		// create a dummy image
		img := createDummyImage(i)
		i = (i + 1) % 3

		// get current timestamp
		pts := multiplyAndDivide(int64(time.Since(start)), int64(forma.ClockRate()), int64(time.Second))

		// encode the image with VP8
		au, pts, err := vp8enc.encode(img, pts)
		if err != nil {
			panic(err)
		}

		// wait for a VP8 access unit
		if au == nil {
			continue
		}

		// generate RTP packets from the VP8 access unit
		pkts, err := rtpEnc.Encode(au)
		if err != nil {
			panic(err)
		}

		log.Printf("writing RTP packets with PTS=%d, au=%d, pkts=%d", pts, len(au), len(pkts))

		// write RTP packets to the server
		for _, pkt := range pkts {
			pkt.Timestamp = uint32(int64(randomStart) + pts)

			err = c.WritePacketRTP(desc.Medias[0], pkt)
			if err != nil {
				panic(err)
			}
		}
	}
}
