package main

import (
	"image"
	"image/jpeg"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's a H264 track
// 3. decode H264 into RGBA frames
// 4. encode the frames into JPEG images and save them on disk

// This example requires the ffmpeg libraries, that can be installed in this way:
// apt install -y libavformat-dev libswscale-dev gcc pkg-config

func saveToFile(img image.Image) error {
	// create file
	fname := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10) + ".jpg"
	f, err := os.Create(fname)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	log.Println("saving", fname)

	// convert to jpeg
	return jpeg.Encode(f, img, &jpeg.Options{
		Quality: 60,
	})
}

func main() {
	c := gortsplib.Client{}

	// parse URL
	u, err := url.Parse("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// find published tracks
	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 track
	track := func() *gortsplib.TrackH264 {
		for _, track := range tracks {
			if track, ok := track.(*gortsplib.TrackH264); ok {
				return track
			}
		}
		return nil
	}()
	if track == nil {
		panic("H264 track not found")
	}

	// setup RTP/H264->H264 decoder
	rtpDec := &rtph264.Decoder{
		PacketizationMode: track.PacketizationMode,
	}
	rtpDec.Init()

	// setup H264->raw frames decoder
	h264RawDec, err := newH264Decoder()
	if err != nil {
		panic(err)
	}
	defer h264RawDec.close()

	// if SPS and PPS are present into the SDP, send them to the decoder
	sps := track.SafeSPS()
	if sps != nil {
		h264RawDec.decode(sps)
	}
	pps := track.SafePPS()
	if pps != nil {
		h264RawDec.decode(pps)
	}

	// called when a RTP packet arrives
	saveCount := 0
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// convert RTP packets into NALUs
		nalus, _, err := rtpDec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		for _, nalu := range nalus {
			// convert NALUs into RGBA frames
			img, err := h264RawDec.decode(nalu)
			if err != nil {
				panic(err)
			}

			// wait for a frame
			if img == nil {
				continue
			}

			// convert frame to JPEG and save to file
			err = saveToFile(img)
			if err != nil {
				panic(err)
			}

			saveCount++
			if saveCount == 5 {
				log.Printf("saved 5 images, exiting")
				os.Exit(1)
			}
		}
	}

	// setup and read the H264 track only
	err = c.SetupAndPlay(gortsplib.Tracks{track}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
