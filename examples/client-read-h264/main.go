package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check whether there's an H264 track
// 3. get H264 NALUs of that track

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
	defer c.Close()

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

	// find the H264 track
	h264Track := func() int {
		for i, track := range tracks {
			if track.IsH264() {
				return i
			}
		}
		return -1
	}()
	if h264Track < 0 {
		panic("H264 track not found")
	}

	// setup decoder
	dec := rtph264.NewDecoder()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(trackID int, payload []byte) {
		if trackID != h264Track {
			return
		}

		// parse RTP packet
		var pkt rtp.Packet
		err := pkt.Unmarshal(payload)
		if err != nil {
			return
		}

		// decode H264 NALUs from RTP packets
		nalus, _, err := dec.Decode(&pkt)
		if err != nil {
			return
		}

		// print NALUs
		for _, nalu := range nalus {
			fmt.Printf("received H264 NALU of size %d\n", len(nalu))
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
