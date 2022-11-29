package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/media"
	"github.com/aler9/gortsplib/pkg/track"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an H264 track
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

	// find the H264 media and track
	var trak *track.H264
	medi := medias.Find(&trak)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP/H264->H264 decoder
	rtpDec := trak.CreateDecoder()

	// setup H264->raw frames decoder
	h264RawDec, err := newH264Decoder()
	if err != nil {
		panic(err)
	}
	defer h264RawDec.close()

	// if SPS and PPS are present into the SDP, send them to the decoder
	sps := trak.SafeSPS()
	if sps != nil {
		h264RawDec.decode(sps)
	}
	pps := trak.SafePPS()
	if pps != nil {
		h264RawDec.decode(pps)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// get packets of specific track only
		if ctx.Packet.PayloadType != trak.PayloadType() {
			return
		}

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

			log.Printf("decoded frame with size %v", img.Bounds().Max)
		}
	}

	// setup and read the H264 media only
	err = c.SetupAndPlay(media.Medias{medi}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
