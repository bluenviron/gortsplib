package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtpmpeg4audio"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's an AAC track
// 3. get AAC AUs of that track

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

	// find the AAC track
	aacTrack, aacTrackID := func() (*gortsplib.TrackMPEG4Audio, int) {
		for i, track := range tracks {
			if tt, ok := track.(*gortsplib.TrackMPEG4Audio); ok {
				return tt, i
			}
		}
		return nil, -1
	}()
	if aacTrack == nil {
		panic("AAC track not found")
	}

	// setup decoder
	dec := &rtpmpeg4audio.Decoder{
		SampleRate:       aacTrack.Config.SampleRate,
		SizeLength:       aacTrack.SizeLength,
		IndexLength:      aacTrack.IndexLength,
		IndexDeltaLength: aacTrack.IndexDeltaLength,
	}
	dec.Init()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		if ctx.TrackID != aacTrackID {
			return
		}

		// decode AAC AUs from the RTP packet
		aus, _, err := dec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		// print AUs
		for _, au := range aus {
			log.Printf("received AAC AU of size %d\n", len(au))
		}
	}

	// setup and read all tracks
	err = c.SetupAndPlay(tracks, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
