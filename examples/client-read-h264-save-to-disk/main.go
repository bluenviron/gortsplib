package main

import (
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/pion/rtp/v2"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's a H264 track
// 3. save the content of the H264 track to a file in MPEG-TS format

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

	// find published tracks
	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 track
	var sps []byte
	var pps []byte
	h264Track := func() int {
		for i, track := range tracks {
			if h264t, ok := track.(*gortsplib.TrackH264); ok {
				sps = h264t.SPS()
				pps = h264t.PPS()
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

	// setup encoder
	enc, err := newMPEGTSEncoder(sps, pps)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP = func(trackID int, pkt *rtp.Packet) {
		if trackID != h264Track {
			return
		}

		// decode H264 NALUs from the RTP packet
		nalus, pts, err := dec.DecodeUntilMarker(pkt)
		if err != nil {
			return
		}

		// encode H264 NALUs into MPEG-TS
		err = enc.encode(nalus, pts)
		if err != nil {
			return
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
