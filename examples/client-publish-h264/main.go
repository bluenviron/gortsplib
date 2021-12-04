package main

import (
	"log"
	"net"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/pion/rtp"
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

	log.Println("Waiting for a RTP/H264 stream on UDP port 9000 - you can send one with Gstreamer:\n" +
		"gst-launch-1.0 videotestsrc ! video/x-raw,width=1920,height=1080" +
		" ! x264enc speed-preset=veryfast tune=zerolatency bitrate=600000" +
		" ! rtph264pay ! udpsink host=127.0.0.1 port=9000")

	// get SPS and PPS
	decoder := rtph264.NewDecoder()
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

	c := gortsplib.Client{}

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
		// read packets from the source
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			panic(err)
		}

		// marshal RTP packets
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
