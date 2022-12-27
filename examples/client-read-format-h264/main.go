package main

import (
	"log"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtph264"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an H264 media
// 3. decode H264 into RGBA frames

// This example requires the ffmpeg libraries, that can be installed in this way:
// apt install -y libavformat-dev libswscale-dev gcc pkg-config

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

	// find published medias
	medias, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 media and format
	var forma *format.H264
	medi := medias.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP/H264->H264 decoder
	rtpDec := forma.CreateDecoder()

	// setup H264->raw frames decoder
	h264RawDec, err := newH264Decoder()
	if err != nil {
		panic(err)
	}
	defer h264RawDec.close()

	// if SPS and PPS are present into the SDP, send them to the decoder
	sps := forma.SafeSPS()
	if sps != nil {
		h264RawDec.decode(sps)
	}
	pps := forma.SafePPS()
	if pps != nil {
		h264RawDec.decode(pps)
	}

	// setup the chosen media only
	_, err = c.Setup(medi, baseURL, 0, 0)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		// convert RTP packets into NALUs
		nalus, pts, err := rtpDec.Decode(pkt)
		if err != nil {
			if err != rtph264.ErrNonStartingPacketAndNoPrevious && err != rtph264.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
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

			log.Printf("decoded frame with size %v and pts %v", img.Bounds().Max, pts)
		}
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
