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
	// connect to the server and start reading all tracks
	conn, tracks, err := gortsplib.DialRead("rtsp://localhost:8554/mystream", gortsplib.StreamProtocolUDP)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	var wg sync.WaitGroup
	defer wg.Wait()
	defer conn.CloseUDPListeners()

	for trackId := range tracks {
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
		}(trackId)

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
		}(trackId)
	}

	err = conn.LoopUDP()
	fmt.Println("connection is closed (%s)", err)
}
