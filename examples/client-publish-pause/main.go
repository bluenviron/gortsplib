package main

import (
	"fmt"
	"net"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
)

// This example shows how to
// 1. generate RTP/H264 frames from a file with Gstreamer
// 2. connect to a RTSP server, announce an H264 track
// 3. write the frames to the server for 5 seconds
// 4. pause for 5 seconds
// 5. repeat

func main() {
	// open a listener to receive RTP/H264 frames
	pc, err := net.ListenPacket("udp", "localhost:9000")
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	fmt.Println("Waiting for a rtp/h264 stream on port 9000 - you can send one with gstreamer:\n" +
		"gst-launch-1.0 filesrc location=video.mp4 ! qtdemux ! video/x-h264" +
		" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=9000")

	// get SPS and PPS
	decoder := rtph264.NewDecoder()
	sps, pps, err := decoder.ReadSPSPPS(rtph264.PacketConnReader{pc})
	if err != nil {
		panic(err)
	}
	fmt.Println("stream connected")

	// create an H264 track
	track, err := gortsplib.NewTrackH264(96, &gortsplib.TrackConfigH264{sps, pps})
	if err != nil {
		panic(err)
	}

	// connect to the server and start publishing the track
	conn, err := gortsplib.DialPublish("rtsp://localhost:8554/mystream",
		gortsplib.Tracks{track})
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for {
		writerDone := make(chan struct{})
		go func() {
			defer close(writerDone)

			buf := make([]byte, 2048)
			for {
				// read packets from the source
				n, _, err := pc.ReadFrom(buf)
				if err != nil {
					break
				}

				// route RTP packets to the server
				err = conn.WriteFrame(0, gortsplib.StreamTypeRTP, buf[:n])
				if err != nil {
					break
				}
			}
		}()

		// wait
		time.Sleep(5 * time.Second)

		// pause
		_, err := conn.Pause()
		if err != nil {
			panic(err)
		}

		// join writer
		<-writerDone

		// wait
		time.Sleep(5 * time.Second)

		// record again
		_, err = conn.Record()
		if err != nil {
			panic(err)
		}
	}
}
