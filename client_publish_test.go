package gortsplib

import (
	"bufio"
	"crypto/tls"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

var testRTPPacket = rtp.Packet{
	Header: rtp.Header{
		Version:     2,
		PayloadType: 97,
		CSRC:        []uint32{},
		SSRC:        0x38F27A2F,
	},
	Payload: []byte{0x01, 0x02, 0x03, 0x04},
}

var testRTPPacketMarshaled = func() []byte {
	byts, _ := testRTPPacket.Marshal()
	return byts
}()

var testRTCPPacket = rtcp.SourceDescription{
	Chunks: []rtcp.SourceDescriptionChunk{
		{
			Source: 1234,
			Items: []rtcp.SourceDescriptionItem{
				{
					Type: rtcp.SDESCNAME,
					Text: "myname",
				},
			},
		},
	},
}

var testRTCPPacketMarshaled = func() []byte {
	byts, _ := testRTCPPacket.Marshal()
	return byts
}()

func TestClientPublishSerial(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
		"tls",
	} {
		t.Run(transport, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			var scheme string
			if transport == "tls" {
				scheme = "rtsps"

				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)

				l = tls.NewListener(l, &tls.Config{Certificates: []tls.Certificate{cert}})
			} else {
				scheme = "rtsp"
			}

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream/trackID=0"), req.URL)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				var l1 net.PacketConn
				var l2 net.PacketConn
				if transport == "udp" {
					l1, err = net.ListenPacket("udp", "localhost:34556")
					require.NoError(t, err)
					defer l1.Close()

					l2, err = net.ListenPacket("udp", "localhost:34557")
					require.NoError(t, err)
					defer l2.Close()
				}

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				// client -> server (RTP)
				if transport == "udp" {
					buf := make([]byte, 2048)
					n, _, err := l1.ReadFrom(buf)
					require.NoError(t, err)
					var pkt rtp.Packet
					err = pkt.Unmarshal(buf[:n])
					require.NoError(t, err)
					require.Equal(t, testRTPPacket, pkt)
				} else {
					var f base.InterleavedFrame
					err = f.Read(1024, br)
					require.NoError(t, err)
					require.Equal(t, 0, f.Channel)
					var pkt rtp.Packet
					err = pkt.Unmarshal(f.Payload)
					require.NoError(t, err)
					require.Equal(t, testRTPPacket, pkt)
				}

				// server -> client (RTCP)
				if transport == "udp" {
					l2.WriteTo(testRTCPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[1],
					})
				} else {
					byts, _ := base.InterleavedFrame{
						Channel: 1,
						Payload: testRTCPPacketMarshaled,
					}.Marshal()
					_, err = conn.Write(byts)
					require.NoError(t, err)
				}

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)
			}()

			recvDone := make(chan struct{})

			c := Client{
				TLSConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
				OnPacketRTCP: func(ctx *ClientOnPacketRTCPCtx) {
					require.Equal(t, 0, ctx.TrackID)
					require.Equal(t, &testRTCPPacket, ctx.Packet)
					close(recvDone)
				},
			}

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			err = c.StartPublishing(scheme+"://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			done := make(chan struct{})
			go func() {
				defer close(done)
				c.Wait()
			}()

			err = c.WritePacketRTP(0, &testRTPPacket, true)
			require.NoError(t, err)

			<-recvDone
			c.Close()
			<-done

			err = c.WritePacketRTP(0, &testRTPPacket, true)
			require.Error(t, err)
		})
	}
}

func TestClientPublishParallel(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
		"tls",
	} {
		t.Run(transport, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			var scheme string
			if transport == "tls" {
				scheme = "rtsps"

				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)

				l = tls.NewListener(l, &tls.Config{Certificates: []tls.Certificate{cert}})
			} else {
				scheme = "rtsp"
			}

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(br)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)
			}()

			c := Client{
				TLSConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			writerDone := make(chan struct{})
			defer func() { <-writerDone }()

			err = c.StartPublishing(scheme+"://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer c.Close()

			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := c.WritePacketRTP(0, &testRTPPacket, true)
					if err != nil {
						return
					}
				}
			}()

			time.Sleep(1 * time.Second)
		})
	}
}

func TestClientPublishPauseSerial(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(br)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(br)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)
			}()

			c := Client{
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			err = c.StartPublishing("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer c.Close()

			err = c.WritePacketRTP(0, &testRTPPacket, true)
			require.NoError(t, err)

			_, err = c.Pause()
			require.NoError(t, err)

			_, err = c.Record()
			require.NoError(t, err)

			err = c.WritePacketRTP(0, &testRTPPacket, true)
			require.NoError(t, err)
		})
	}
}

func TestClientPublishPauseParallel(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(br)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)
			}()

			c := Client{
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			err = c.StartPublishing("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			writerDone := make(chan struct{})
			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := c.WritePacketRTP(0, &testRTPPacket, true)
					if err != nil {
						return
					}
				}
			}()

			time.Sleep(1 * time.Second)

			_, err = c.Pause()
			require.NoError(t, err)

			c.Close()
			<-writerDone
		})
	}
}

func TestClientPublishAutomaticProtocol(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusUnsupportedTransport,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)
		require.Equal(t, headers.TransportProtocolTCP, inTH.Protocol)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		}

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Record, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		var f base.InterleavedFrame
		err = f.Read(2048, br)
		require.NoError(t, err)
		require.Equal(t, 0, f.Channel)
		var pkt rtp.Packet
		err = pkt.Unmarshal(f.Payload)
		require.NoError(t, err)
		require.Equal(t, testRTPPacket, pkt)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	track := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	c := Client{}

	err = c.StartPublishing("rtsp://localhost:8554/teststream",
		Tracks{track})
	require.NoError(t, err)
	defer c.Close()

	err = c.WritePacketRTP(0, &testRTPPacket, true)
	require.NoError(t, err)
}

func TestClientPublishRTCPReport(t *testing.T) {
	reportReceived := make(chan struct{})

	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		l1, err := net.ListenPacket("udp", "localhost:34556")
		require.NoError(t, err)
		defer l1.Close()

		l2, err := net.ListenPacket("udp", "localhost:34557")
		require.NoError(t, err)
		defer l2.Close()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
					Protocol:    headers.TransportProtocolUDP,
					ClientPorts: inTH.ClientPorts,
					ServerPorts: &[2]int{34556, 34557},
				}.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Record, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		buf := make([]byte, 2048)
		n, _, err := l1.ReadFrom(buf)
		require.NoError(t, err)
		var pkt rtp.Packet
		err = pkt.Unmarshal(buf[:n])
		require.NoError(t, err)

		buf = make([]byte, 2048)
		n, _, err = l2.ReadFrom(buf)
		require.NoError(t, err)
		packets, err := rtcp.Unmarshal(buf[:n])
		require.NoError(t, err)
		sr, ok := packets[0].(*rtcp.SenderReport)
		require.True(t, ok)
		require.Equal(t, &rtcp.SenderReport{
			SSRC:        753621,
			NTPTime:     sr.NTPTime,
			RTPTime:     sr.RTPTime,
			PacketCount: 1,
			OctetCount:  4,
		}, sr)

		close(reportReceived)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	c := Client{
		udpSenderReportPeriod: 1 * time.Second,
	}

	track := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	err = c.StartPublishing("rtsp://localhost:8554/teststream",
		Tracks{track})
	require.NoError(t, err)
	defer c.Close()

	err = c.WritePacketRTP(0, &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      54352,
			SSRC:           753621,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}, true)
	require.NoError(t, err)

	<-reportReceived
}

func TestClientPublishIgnoreTCPRTPPackets(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Record, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		byts, _ = base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		byts, _ = base.InterleavedFrame{
			Channel: 1,
			Payload: testRTCPPacketMarshaled,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	rtcpReceived := make(chan struct{})

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
		OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
			t.Errorf("should not happen")
		},
		OnPacketRTCP: func(ctx *ClientOnPacketRTCPCtx) {
			close(rtcpReceived)
		},
	}

	track := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	err = c.StartPublishing("rtsp://localhost:8554/teststream",
		Tracks{track})
	require.NoError(t, err)
	defer c.Close()

	<-rtcpReceived
}
