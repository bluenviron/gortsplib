// +build ignore

package main

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/aler9/gortsplib"
)

func main() {
	u, err := url.Parse("rtsp://user:pass@example.com/mystream")
	if err != nil {
		panic(err)
	}

	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	rconn, err := gortsplib.NewConnClient(gortsplib.ConnClientConf{Conn: conn})
	if err != nil {
		panic(err)
	}

	_, err = rconn.Options(u)
	if err != nil {
		panic(err)
	}

	sdpd, _, err := rconn.Describe(u)
	if err != nil {
		panic(err)
	}

	for i, media := range sdpd.MediaDescriptions {
		_, err := rconn.SetupTcp(u, media, i)
		if err != nil {
			panic(err)
		}
	}

	_, err = rconn.Play(u)
	if err != nil {
		panic(err)
	}

	frame := &gortsplib.InterleavedFrame{Content: make([]byte, 512*1024)}

	for {
		err := rconn.ReadFrame(frame)
		if err != nil {
			panic(err)
		}

		trackId, streamType := gortsplib.ConvChannelToTrackIdAndStreamType(frame.Channel)
		fmt.Printf("packet from track %d, type %v: %v\n", trackId, streamType, frame.Content)
	}
}
