package main

import (
	"fmt"
	"net"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
)

// This example shows how to
// 1. generate RTP/H264 packets with Gstreamer
// 2. connect to a RTSP server, announce an H264 track
// 3. route the packets from Gstreamer to the server

func main() {
	// open a listener to receive RTP/H264 packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	fmt.Println("Waiting for a RTP/h264 stream on UDP port 9000 - you can send one with Gstreamer:\n" +
		"gst-launch-1.0 videotestsrc ! video/x-raw,width=1920,height=1080" +
		" ! x264enc speed-preset=veryfast tune=zerolatency bitrate=600000" +
		" ! rtph264pay ! udpsink host=127.0.0.1 port=9000")

	// get SPS and PPS
	decoder := rtph264.NewDecoder()
	sps, pps, err := decoder.ReadSPSPPS(rtph264.PacketConnReader{pc})
	if err != nil {
		panic(err)
	}
	fmt.Println("stream connected")

	// create an H264 track
	track, err := gortsplib.NewTrackH264(96, &gortsplib.TrackConfigH264{sps, pps})
	if err != nil {
		panic(err)
	}

	// connect to the server and start publishing the track
	conn, err := gortsplib.DialPublish("rtsp://localhost:8554/mystream",
		gortsplib.Tracks{track})
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	buf := make([]byte, 2048)
	for {
		// read RTP packets from the source
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			panic(err)
		}

		// write RTP packets
		err = conn.WriteFrame(0, gortsplib.StreamTypeRTP, buf[:n])
		if err != nil {
			panic(err)
		}
	}
}
