package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check whether there's an H264 track
// 3. get H264 NALUs of that track

func main() {
	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// find the H264 track
	h264Track := func() int {
		for i, track := range conn.Tracks() {
			if track.IsH264() {
				return i
			}
		}
		return -1
	}()
	if h264Track < 0 {
		panic(fmt.Errorf("H264 track not found"))
	}
	fmt.Printf("H264 track is number %d\n", h264Track+1)

	// instantiate a RTP/H264 decoder
	dec := rtph264.NewDecoder()

	// read packets
	err = conn.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
		if streamType != gortsplib.StreamTypeRTP {
			return
		}

		if trackID != h264Track {
			return
		}

		// parse RTP packets
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
	})
	panic(err)
}
