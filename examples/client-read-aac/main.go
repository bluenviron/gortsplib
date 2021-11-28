package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/rtpaac"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check whether there's an AAC track
// 3. get AAC AUs of that track

func main() {
	c := gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}

	// get available methods
	_, err = c.Options(u)
	if err != nil {
		panic(err)
	}

	// find published tracks
	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the AAC track
	aacTrack := func() int {
		for i, track := range tracks {
			if track.IsAAC() {
				return i
			}
		}
		return -1
	}()
	if aacTrack < 0 {
		panic("AAC track not found")
	}

	// get track config
	aacConf, err := tracks[aacTrack].ExtractConfigAAC()
	if err != nil {
		panic(err)
	}

	// setup decoder
	dec := rtpaac.NewDecoder(aacConf.SampleRate)

	// called when a RTP packet arrives
	c.OnPacketRTP = func(trackID int, payload []byte) {
		if trackID != aacTrack {
			return
		}

		// parse RTP packet
		var pkt rtp.Packet
		err := pkt.Unmarshal(payload)
		if err != nil {
			return
		}

		// decode AAC AUs from RTP packets
		aus, _, err := dec.Decode(&pkt)
		if err != nil {
			return
		}

		// print AUs
		for _, au := range aus {
			fmt.Printf("received AAC AU of size %d\n", len(au))
		}
	}

	// start reading tracks
	err = c.SetupAndPlay(tracks, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
