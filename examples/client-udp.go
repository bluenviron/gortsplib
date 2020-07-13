// +build ignore

package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
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

	rconn := gortsplib.NewConnClient(gortsplib.ConnClientConf{Conn: conn})

	_, err = rconn.Options(u)
	if err != nil {
		panic(err)
	}

	sdpd, _, err := rconn.Describe(u)
	if err != nil {
		panic(err)
	}

	var rtpListeners []net.PacketConn
	var rtcpListeners []net.PacketConn

	for i, media := range sdpd.MediaDescriptions {
		rtpPort := 9000 + i*2
		rtpl, err := net.ListenPacket("udp", ":"+strconv.FormatInt(int64(rtpPort), 10))
		if err != nil {
			panic(err)
		}
		defer rtpl.Close()
		rtpListeners = append(rtpListeners, rtpl)

		rtcpPort := 9001 + i*2
		rtcpl, err := net.ListenPacket("udp", ":"+strconv.FormatInt(int64(rtcpPort), 10))
		if err != nil {
			panic(err)
		}
		defer rtcpl.Close()
		rtcpListeners = append(rtcpListeners, rtcpl)

		_, _, _, err = rconn.SetupUdp(u, media, rtpPort, rtcpPort)
		if err != nil {
			panic(err)
		}
	}

	_, err = rconn.Play(u)
	if err != nil {
		panic(err)
	}

	done := make(chan struct{})

	// send periodic keepalive
	go func() {
		keepaliveTicker := time.NewTicker(30 * time.Second)
		for range keepaliveTicker.C {
			_, err = rconn.Options(u)
			if err != nil {
				fmt.Println("connection is closed")
				close(done)
				break
			}
		}
	}()

	// receive RTP packets
	for trackId, l := range rtpListeners {
		go func(trackId int, l net.PacketConn) {
			buf := make([]byte, 2048)
			for {
				n, _, err := l.ReadFrom(buf)
				if err != nil {
					break
				}

				fmt.Printf("packet from track %d, type RTP: %v\n", trackId, buf[:n])
			}
		}(trackId, l)
	}

	// receive RTCP packets
	for trackId, l := range rtcpListeners {
		go func(trackId int, l net.PacketConn) {
			buf := make([]byte, 2048)
			for {
				n, _, err := l.ReadFrom(buf)
				if err != nil {
					break
				}

				fmt.Printf("packet from track %d, type RTCP: %v\n", trackId, buf[:n])
			}
		}(trackId, l)
	}

	<-done
}
