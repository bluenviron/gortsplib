package main

import (
	"log"
	"net"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. generate RTP/VP9 packets with GStreamer
// 2. connect to a RTSP server, announce a VP9 media
// 3. route the packets from GStreamer to the server

func main() {
	// open a listener to receive RTP/VP9 packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a RTP/VP9 stream on UDP port 9000 - you can send one with GStreamer:\n" +
		"gst-launch-1.0 videotestsrc ! video/x-raw,width=1920,height=1080" +
		" ! vp9enc cpu-used=8 deadline=1" +
		" ! rtpvp9pay ! udpsink host=127.0.0.1 port=9000")

	// wait for first packet
	buf := make([]byte, 2048)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	// create a media that contains a VP9 format
	medi := &media.Media{
		Type: media.TypeVideo,
		Formats: []format.Format{&format.VP9{
			PayloadTyp: 96,
		}},
	}

	// connect to the server and start recording the media
	c := gortsplib.Client{}
	err = c.StartRecording("rtsp://localhost:8554/mystream",
		media.Medias{medi})
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
		err = c.WritePacketRTP(medi, &pkt)
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
