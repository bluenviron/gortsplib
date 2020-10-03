// +build ignore

package main

import (
	"fmt"
	"net"
	"net/url"

	"github.com/aler9/gortsplib"
	"github.com/pion/rtp"
)

// This example shows how to generate RTP/H264 frames from a file with Gstreamer,
// create a RTSP client, connect to a server, announce a H264 track and write
// the frames with the UDP protocol.

func getRtpH264SPSandPPS(pc net.PacketConn) ([]byte, []byte, error) {
	var sps []byte
	var pps []byte

	buf := make([]byte, 2048)
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return nil, nil, err
		}

		pkt := &rtp.Packet{}
		err = pkt.Unmarshal(buf[:n])
		if err != nil {
			return nil, nil, err
		}

		// require h264
		if pkt.PayloadType != 96 {
			return nil, nil, fmt.Errorf("wrong payload type '%d', expected 96",
				pkt.PayloadType)
		}

		// switch by NALU type
		switch pkt.Payload[0] & 0x1F {
		case 0x07: // sps
			sps = append([]byte(nil), pkt.Payload...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}

		case 0x08: // pps
			pps = append([]byte(nil), pkt.Payload...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}
		}
	}
}

func main() {
	// open a listener to receive RTP/H264 frames
	pc, err := net.ListenPacket("udp4", "127.0.0.1:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	fmt.Println("Waiting for a rtp/h264 stream on port 9000 - you can send one with gstreamer:\n" +
		"gst-launch-1.0 filesrc location=video.mp4 ! qtdemux ! video/x-h264" +
		" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=9000")

	// wait for RTP/H264 frames
	sps, pps, err := getRtpH264SPSandPPS(pc)
	if err != nil {
		panic(err)
	}
	fmt.Println("stream connected")

	// parse url
	u, err := url.Parse("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	conn, err := gortsplib.NewConnClient(gortsplib.ConnClientConf{Host: u.Host})
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// get allowed commands
	_, err = conn.Options(u)
	if err != nil {
		panic(err)
	}

	// create a H264 track
	track, err := gortsplib.NewTrackH264(0, sps, pps)
	if err != nil {
		panic(err)
	}

	// announce the track
	_, err = conn.Announce(u, gortsplib.Tracks{track})
	if err != nil {
		panic(err)
	}

	// setup the track with UDP
	_, err = conn.SetupUDP(u, gortsplib.SetupModeRecord, track, 0, 0)
	if err != nil {
		panic(err)
	}

	// start publishing
	_, err = conn.Record(u)
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 2048)
	for {
		// read frames from the source
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			break
		}

		// write frames to the server
		err = conn.WriteFrameUDP(track, gortsplib.StreamTypeRtp, buf[:n])
		if err != nil {
			break
		}
	}
}
