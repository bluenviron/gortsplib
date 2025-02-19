package main

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	"image/jpeg"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
)

// This example shows how to
// 1. connect to a RTSP server, announce a M-JPEG format
// 2. generate dummy RGBA images
// 3. encode images with JPEG
// 4. generate RTP packets from JPEG
// 5. write RTP packets to the server

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
	// create a description that contains a M-JPEG format
	forma := &format.MJPEG{}
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

	// setup JPEG -> RTP encoder
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

		// encode the image with JPEG
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
		if err != nil {
			panic(err)
		}

		// generate RTP packets from the JPEG image
		pkts, err := rtpEnc.Encode(buf.Bytes())
		if err != nil {
			panic(err)
		}

		// get current timestamp
		pts := multiplyAndDivide(int64(time.Since(start)), int64(forma.ClockRate()), int64(time.Second))

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
