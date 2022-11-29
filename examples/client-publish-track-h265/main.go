package main

import (
	"log"
	"net"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/track"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. generate RTP/H265 packets with GStreamer
// 2. connect to a RTSP server, announce an H265 track
// 3. route the packets from GStreamer to the server

func main() {
	// open a listener to receive RTP/H265 packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a RTP/H265 stream on UDP port 9000 - you can send one with GStreamer:\n" +
		"gst-launch-1.0 videotestsrc ! video/x-raw,width=1920,height=1080" +
		" ! x265enc speed-preset=ultrafast tune=zerolatency bitrate=6000" +
		" ! rtph265pay config-interval=1 ! udpsink host=127.0.0.1 port=9000")

	// wait for first packet
	buf := make([]byte, 2048)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	// create a media that contains a H265 track
	media := &gortsplib.Media{
		Type: gortsplib.MediaTypeVideo,
		Tracks: []track.Track{&track.H265{
			PayloadTyp: 96,
		}},
	}

	// connect to the server and start publishing the media
	c := gortsplib.Client{}
	err = c.StartPublishing("rtsp://localhost:8554/mystream",
		gortsplib.Medias{media})
	if err != nil {
		panic(err)
	}
	defer c.Close()

	var pkt rtp.Packet
	for {
		// parse RTP packet
		err = pkt.Unmarshal(buf[:n])
		if err != nil {
			panic(err)
		}

		// route RTP packet to the server
		err = c.WritePacketRTP(0, &pkt)
		if err != nil {
			panic(err)
		}

		// read another RTP packet from source
		n, _, err = pc.ReadFrom(buf)
		if err != nil {
			panic(err)
		}
	}
}
