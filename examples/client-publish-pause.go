// +build ignore

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
// 2. connect to a RTSP server, announce a H264 track
// 3. write the frames to the server for 5 seconds
// 4. pause for 5 seconds
// 5. repeat

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
	decoder := rtph264.NewDecoderFromPacketConn(pc)
	sps, pps, err := decoder.ReadSPSPPS()
	if err != nil {
		panic(err)
	}
	fmt.Println("stream connected")

	// create a H264 track
	track, err := gortsplib.NewTrackH264(96, sps, pps)
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
				// read frames from the source
				n, _, err := pc.ReadFrom(buf)
				if err != nil {
					break
				}

				// write track frames
				err = conn.WriteFrame(track.ID, gortsplib.StreamTypeRTP, buf[:n])
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
