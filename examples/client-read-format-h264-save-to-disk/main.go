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
	var forma *format.H264
	medi := medias.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP/H264->H264 decoder
	rtpDec := forma.CreateDecoder()

	// setup H264->MPEGTS muxer
	mpegtsMuxer, err := newMPEGTSMuxer(forma.SafeSPS(), forma.SafePPS())
	if err != nil {
		panic(err)
	}

	// setup the chosen media only
	_, err = c.Setup(medi, baseURL, 0, 0)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		// convert RTP packets into NALUs
		// DecodeUntilMarker is necessary for the DTS extractor to work
		nalus, pts, err := rtpDec.DecodeUntilMarker(pkt)
		if err != nil {
			if err != rtph264.ErrNonStartingPacketAndNoPrevious && err != rtph264.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
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
