package main

import (
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/media"
	"github.com/aler9/gortsplib/pkg/track"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a H264 track
// 3. save the content of the H264 track into a file in MPEG-TS format

func findTrack(medias media.Medias) (*media.Media, *track.H264) {
	for _, media := range medias {
		for _, trak := range media.Tracks {
			if trak, ok := trak.(*track.H264); ok {
				return media, trak
			}
		}
	}
	return nil, nil
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

	// find published medias
	medias, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 media and track
	medi, track := findTrack(medias)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP/H264->H264 decoder
	rtpDec := track.CreateDecoder()

	// setup H264->MPEGTS muxer
	mpegtsMuxer, err := newMPEGTSMuxer(track.SafeSPS(), track.SafePPS())
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// get packets of specific track only
		if ctx.Packet.PayloadType != track.PayloadType() {
			return
		}

		// convert RTP packets into NALUs
		nalus, pts, err := rtpDec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		// encode H264 NALUs into MPEG-TS
		mpegtsMuxer.encode(nalus, pts)
	}

	// setup and read the H264 media only
	err = c.SetupAndPlay(media.Medias{medi}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
