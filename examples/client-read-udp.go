// +build ignore

package main

import (
	"fmt"
	"sync"

	"github.com/aler9/gortsplib"
)

// This example shows how to create a RTSP client, connect to a server and
// read all tracks with the UDP protocol.

func main() {
	var wg sync.WaitGroup
	defer wg.Wait()

	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead("rtsp://localhost:8554/mystream", gortsplib.StreamProtocolUDP)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for _, track := range conn.Tracks() {
		// read RTP frames
		wg.Add(1)
		go func(trackId int) {
			defer wg.Done()

			for {
				buf, err := conn.ReadFrameUDP(trackId, gortsplib.StreamTypeRtp)
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTP: %v\n", trackId, buf)
			}
		}(track.Id)

		// read RTCP frames
		wg.Add(1)
		go func(trackId int) {
			defer wg.Done()

			for {
				buf, err := conn.ReadFrameUDP(trackId, gortsplib.StreamTypeRtcp)
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTCP: %v\n", trackId, buf)
			}
		}(track.Id)
	}

	// wait until the connection is closed
	err = conn.LoopUDP()
	fmt.Println("connection is closed (%s)", err)
}
