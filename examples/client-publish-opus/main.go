package main

import (
	"log"
	"net"

	"github.com/aler9/gortsplib"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. generate RTP/Opus packets with Gstreamer
// 2. connect to a RTSP server, announce an Opus track
// 3. route the packets from Gstreamer to the server

func main() {
	// open a listener to receive RTP/Opus packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a RTP/Opus stream on UDP port 9000 - you can send one with Gstreamer:\n" +
		"gst-launch-1.0 audiotestsrc freq=300 ! audioconvert ! audioresample ! audio/x-raw,rate=48000" +
		" ! opusenc" +
		" ! rtpopuspay ! udpsink host=127.0.0.1 port=9000")

	// wait for first packet
	buf := make([]byte, 2048)
	_, _, err = pc.ReadFrom(buf)
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	// create an Opus track
	track, err := gortsplib.NewTrackOpus(96, 48000, 2)
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

	buf = make([]byte, 2048)
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
