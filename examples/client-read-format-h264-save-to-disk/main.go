package main

import (
	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a H264 media
// 3. save the content of the H264 media into a file in MPEG-TS format

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
	var trak *format.H264
	medi := medias.Find(&trak)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP/H264->H264 decoder
	rtpDec := trak.CreateDecoder()

	// setup H264->MPEGTS muxer
	mpegtsMuxer, err := newMPEGTSMuxer(trak.SafeSPS(), trak.SafePPS())
	if err != nil {
		panic(err)
	}

	// setup the chosen media only
	_, err = c.Setup(medi, baseURL, 0, 0)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP(medi, trak, func(pkt *rtp.Packet) {
		// convert RTP packets into NALUs
		nalus, pts, err := rtpDec.Decode(pkt)
		if err != nil {
			return
		}

		// encode H264 NALUs into MPEG-TS
		mpegtsMuxer.encode(nalus, pts)
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
