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
		_, err := rconn.Setup(u, media, []string{
			"RTP/AVP/TCP",
			"unicast",
			fmt.Sprintf("interleaved=%d-%d", (i * 2), (i*2)+1),
		})
		if err != nil {
			panic(err)
		}
	}

	_, err = rconn.Play(u)
	if err != nil {
		panic(err)
	}

	frame := &gortsplib.InterleavedFrame{
		Content: make([]byte, 512*1024),
	}

	for {
		err := rconn.ReadFrame(frame)
		if err != nil {
			panic(err)
		}

		fmt.Println("incoming", frame.Channel, frame.Content)
	}
}
