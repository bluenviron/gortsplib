package main

import (
	"log"
	"net"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. generate RTP/G711 packets with GStreamer
// 2. connect to a RTSP server, find a back channel that supports G711
// 3. route the packets from GStreamer to the channel

func findPCMUBackChannel(desc *description.Session) *description.Media {
	for _, media := range desc.Medias {
		if media.IsBackChannel {
			for _, forma := range media.Formats {
				if g711, ok := forma.(*format.G711); ok {
					if g711.MULaw {
						return media
					}
				}
			}
		}
	}
	return nil
}

func main() {
	// open a listener to receive RTP/G711 packets
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	log.Println("Waiting for a RTP/G711 stream on UDP port 9000 - you can generate one with GStreamer:\n\n" +
		"* audio from a test sine:\n\n" +
		"gst-launch-1.0 audiotestsrc freq=300 ! audioconvert ! audioresample ! audio/x-raw,rate=8000" +
		" ! mulawenc ! rtppcmupay ! udpsink host=127.0.0.1 port=9000\n\n" +
		"* audio from a file:\n\n" +
		"gst-launch-1.0 filesrc location=my_file.mp4 ! decodebin ! audioconvert ! audioresample ! audio/x-raw,rate=8000" +
		" ! mulawenc ! rtppcmupay ! udpsink host=127.0.0.1 port=9000\n\n" +
		"* audio from a microphone:\n\n" +
		"gst-launch-1.0 pulsesrc ! audioconvert ! audioresample ! audio/x-raw,rate=8000" +
		" ! mulawenc ! rtppcmupay ! udpsink host=127.0.0.1 port=9000\n")

	// wait for first packet
	buf := make([]byte, 2048)
	n, _, err := pc.ReadFrom(buf)
	if err != nil {
		panic(err)
	}
	log.Println("stream connected")

	c := gortsplib.Client{
		RequestBackChannels: true,
	}

	// parse URL
	u, err := base.ParseURL("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// find published medias
	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the back channel
	medi := findPCMUBackChannel(desc)
	if medi == nil {
		panic("media not found")
	}

	// setup a single media
	_, err = c.Setup(desc.BaseURL, medi, 0, 0)
	if err != nil {
		panic(err)
	}

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

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
