package main

import (
	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check whether there's a H264 track
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

	// get track config
	h264Conf, err := tracks[h264Track].ExtractConfigH264()
	if err != nil {
		panic(err)
	}

	// setup decoder
	dec := rtph264.NewDecoder()

	// setup encoder
	enc, err := newMPEGTSEncoder(h264Conf)
	if err != nil {
		panic(err)
	}

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
		nalus, pts, err := dec.DecodeUntilMarker(&pkt)
		if err != nil {
			return
		}

		// encode H264 NALUs into MPEG-TS
		err = enc.encode(nalus, pts)
		if err != nil {
			return
		}
	}

	// setup all tracks
	for _, t := range tracks {
		_, err := c.Setup(true, baseURL, t, 0, 0)
		if err != nil {
			panic(err)
		}
	}

	// start reading tracks
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
