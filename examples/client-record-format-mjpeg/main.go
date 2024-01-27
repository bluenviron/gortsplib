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
// 1. generate RTP/M-JPEG packets with GStreamer
// 2. connect to a RTSP server, announce a M-JPEG format
// 3. route the packets from GStreamer to the server

func main() {
	// open a listener to receive RTP/M-JPEG packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a RTP/M-JPEG stream on UDP port 9000 - you can send one with GStreamer:\n" +
		"gst-launch-1.0 videotestsrc ! video/x-raw,width=1920,height=1080,format=I420" +
		" ! jpegenc ! rtpjpegpay ! udpsink host=127.0.0.1 port=9000")

	// wait for first packet
	buf := make([]byte, 2048)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	// create a description that contains a M-JPEG format
	desc := &description.Session{
		Medias: []*description.Media{{
			Type:    description.MediaTypeVideo,
			Formats: []format.Format{&format.MJPEG{}},
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
