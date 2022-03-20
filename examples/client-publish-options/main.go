package main

import (
	"log"
	"net"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/pion/rtp/v2"
)

// This example shows how to
// 1. set additional client options
// 2. generate RTP/H264 frames from a file with GStreamer
// 3. connect to a RTSP server, announce an H264 track
// 4. write the frames to the server

func main() {
	// open a listener to receive RTP/H264 frames
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a rtp/h264 stream on port 9000 - you can send one with gstreamer:\n" +
		"gst-launch-1.0 filesrc location=video.mp4 ! qtdemux ! video/x-h264" +
		" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=9000")

	// get SPS and PPS
	decoder := &rtph264.Decoder{}
	decoder.Init()
	sps, pps, err := decoder.ReadSPSPPS(rtph264.PacketConnReader{pc})
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	// create an H264 track
	track, err := gortsplib.NewTrackH264(96, sps, pps, nil)
	if err != nil {
		panic(err)
	}

	// Client allows to set additional client options
	c := &gortsplib.Client{
		// the stream transport (UDP or TCP). If nil, it is chosen automatically
		Transport: nil,
		// timeout of read operations
		ReadTimeout: 10 * time.Second,
		// timeout of write operations
		WriteTimeout: 10 * time.Second,
	}

	// connect to the server and start publishing the track
	err = c.StartPublishing("rtsp://localhost:8554/mystream",
		gortsplib.Tracks{track})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	buf := make([]byte, 2048)
	var pkt rtp.Packet
	for {
		// read RTP packets from the source
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			panic(err)
		}

		// parse RTP packets
		err = pkt.Unmarshal(buf[:n])
		if err != nil {
			panic(err)
		}

		// route RTP packets to the server
		err = c.WritePacketRTP(0, &pkt)
		if err != nil {
			panic(err)
		}
	}
}
