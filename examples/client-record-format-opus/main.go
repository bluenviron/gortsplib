package main

import (
	"log"
	"net"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. generate RTP/Opus packets with GStreamer
// 2. connect to a RTSP server, announce an Opus format
// 3. route the packets from GStreamer to the server

func main() {
	// open a listener to receive RTP/Opus packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a RTP/Opus stream on UDP port 9000 - you can send one with GStreamer:\n" +
		"gst-launch-1.0 audiotestsrc freq=300 ! audioconvert ! audioresample ! audio/x-raw,rate=48000" +
		" ! opusenc ! rtpopuspay ! udpsink host=127.0.0.1 port=9000")

	// wait for first packet
	buf := make([]byte, 2048)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	// create a description that contains a Opus format
	desc := &description.Session{
		Medias: []*description.Media{{
			Type: description.MediaTypeVideo,
			Formats: []format.Format{&format.Opus{
				PayloadTyp: 96,
				IsStereo:   false,
			}},
		}},
	}

	// connect to the server and start recording
	c := gortsplib.Client{}
	err = c.StartRecording("rtsp://localhost:8554/mystream", desc)
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
		err = c.WritePacketRTP(desc.Medias[0], &pkt)
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
