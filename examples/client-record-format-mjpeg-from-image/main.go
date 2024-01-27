package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/rtptime"
)

// This example shows how to
// 1. connect to a RTSP server, announce a M-JPEG format
// 2. generate an image
// 3. encode the image with JPEG
// 4. generate RTP/M-JPEG packets from the JPEG image
// 5. write packets to the server

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
	err := c.StartRecording("rtsp://localhost:8554/mystream", desc)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// setup JPEG -> RTP/M-JPEG encoder
	rtpEnc, err := forma.CreateEncoder()
	if err != nil {
		panic(err)
	}

	// setup timestamp generator
	rtpTime := rtptime.NewEncoder(forma.ClockRate(), 0)
	start := time.Now()

	// setup a ticker to sleep between frames
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	i := 0

	for range ticker.C {
		// create a RGBA image
		image := image.NewRGBA(image.Rect(0, 0, 640, 480))

		// fill the image
		var cl color.RGBA
		switch i {
		case 0:
			cl = color.RGBA{255, 0, 0, 0}
		case 1:
			cl = color.RGBA{0, 255, 0, 0}
		case 2:
			cl = color.RGBA{0, 0, 255, 0}
		}
		for y := 0; y < image.Rect.Dy(); y++ {
			for x := 0; x < image.Rect.Dx(); x++ {
				image.SetRGBA(x, y, cl)
			}
		}
		i = (i + 1) % 3

		// encode the image with JPEG
		var buf bytes.Buffer
		err := jpeg.Encode(&buf, image, &jpeg.Options{Quality: 80})
		if err != nil {
			panic(err)
		}

		// generate RTP/M-JPEG packets from the JPEG image
		pkts, err := rtpEnc.Encode(buf.Bytes())
		if err != nil {
			panic(err)
		}

		// get current timestamp
		ts := rtpTime.Encode(time.Now().Sub(start))

		// write packets to the server
		for _, pkt := range pkts {
			pkt.Timestamp = ts

			err = c.WritePacketRTP(desc.Medias[0], pkt)
			if err != nil {
				panic(err)
			}
		}
	}
}
