package main

import (
	"log"
	"net"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/codecs/mpeg4audio"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. generate RTP/MPEG4-audio packets with GStreamer
// 2. connect to a RTSP server, announce an MPEG4-audio media
// 3. route the packets from GStreamer to the server

func main() {
	// open a listener to receive RTP/MPEG4-audio packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a RTP/MPEG4-audio stream on UDP port 9000 - you can send one with GStreamer:\n" +
		"gst-launch-1.0 audiotestsrc freq=300 ! audioconvert ! audioresample ! audio/x-raw,rate=48000" +
		" ! avenc_aac bitrate=128000 ! rtpmp4gpay ! udpsink host=127.0.0.1 port=9000")

	// wait for first packet
	buf := make([]byte, 2048)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	// create a media that contains a MPEG4-audio format
	medias := media.Medias{&media.Media{
		Type: media.TypeAudio,
		Formats: []format.Format{&format.MPEG4Audio{
			PayloadTyp: 96,
			Config: &mpeg4audio.Config{
				Type:         mpeg4audio.ObjectTypeAACLC,
				SampleRate:   48000,
				ChannelCount: 2,
			},
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		}},
	}}

	// connect to the server and start recording the media
	c := gortsplib.Client{}
	err = c.StartRecording("rtsp://localhost:8554/mystream", medias)
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
		err = c.WritePacketRTP(medias[0], &pkt)
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
