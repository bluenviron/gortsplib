// +build ignore

package main

import (
	"fmt"
	"net"
	"net/url"

	"github.com/aler9/gortsplib"
	"github.com/pion/rtp"
)

func getH264SPSandPPS(pc net.PacketConn) ([]byte, []byte, error) {
	var sps []byte
	var pps []byte

	buf := make([]byte, 2048)
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			return nil, nil, err
		}

		packet := &rtp.Packet{}
		err = packet.Unmarshal(buf[:n])
		if err != nil {
			return nil, nil, err
		}

		// require h264
		if packet.PayloadType != 96 {
			return nil, nil, fmt.Errorf("wrong payload type '%d', expected 96",
				packet.PayloadType)
		}

		// switch by NALU type
		switch packet.Payload[0] & 0x1F {
		case 0x07: // sps
			sps = append([]byte(nil), packet.Payload...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}

		case 0x08: // pps
			pps = append([]byte(nil), packet.Payload...)
			if sps != nil && pps != nil {
				return sps, pps, nil
			}
		}
	}
}

func main() {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	fmt.Println("Waiting for a rtp/h264 stream on port 9000 - you can send one with gstreamer:\n" +
		"gst-launch-1.0 filesrc location=video.mp4 ! qtdemux ! video/x-h264" +
		" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=9000")

	sps, pps, err := getH264SPSandPPS(pc)
	if err != nil {
		panic(err)
	}

	fmt.Println("stream is ok")

	u, err := url.Parse("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	conn, err := gortsplib.NewConnClient(gortsplib.ConnClientConf{Host: u.Host})
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Options(u)
	if err != nil {
		panic(err)
	}

	track := gortsplib.NewTrackH264(0, sps, pps)

	_, err = conn.Announce(u, gortsplib.Tracks{track})
	if err != nil {
		panic(err)
	}

	_, err = conn.SetupUDP(u, gortsplib.SetupModeRecord, track, 0, 0)
	if err != nil {
		panic(err)
	}

	_, err = conn.Record(u)
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 2048)
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			break
		}

		err = conn.WriteFrameUDP(track, gortsplib.StreamTypeRtp, buf[:n])
		if err != nil {
			break
		}
	}
}
