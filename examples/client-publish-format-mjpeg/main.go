package main

import (
	"log"
	"net"

	"github.com/bluenviron/gortsplib/v3"
	"github.com/bluenviron/gortsplib/v3/pkg/formats"
	"github.com/bluenviron/gortsplib/v3/pkg/media"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. generate RTP/M-JPEG packets with GStreamer
// 2. connect to a RTSP server, announce a M-JPEG media
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

	// create a media that contains a M-JPEG format
	medi := &media.Media{
		Type:    media.TypeVideo,
		Formats: []formats.Format{&formats.MJPEG{}},
	}

	// connect to the server and start recording the media
	c := gortsplib.Client{}
	err = c.StartRecording("rtsp://localhost:8554/mystream", media.Medias{medi})
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
