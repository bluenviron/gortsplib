package gortsplib

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/rtcpreceiver"
)

func TestClientPublishSerial(t *testing.T) {
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
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				req, err := readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

				var inTH headers.Transport
				err = inTH.Read(req.Header["Transport"])
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

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
					},
				}.Write(bconn.Writer)
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

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				// client -> server
				if transport == "udp" {
					buf := make([]byte, 2048)
					n, _, err := l1.ReadFrom(buf)
					require.NoError(t, err)
					require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, buf[:n])
				} else {
					var f base.InterleavedFrame
					f.Payload = make([]byte, 2048)
					err = f.Read(bconn.Reader)
					require.NoError(t, err)
					require.Equal(t, 0, f.Channel)
					require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, f.Payload)
				}

				// server -> client (RTCP)
				if transport == "udp" {
					l2.WriteTo([]byte{0x05, 0x06, 0x07, 0x08}, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[1],
					})
				} else {
					err = base.InterleavedFrame{
						Channel: 1,
						Payload: []byte{0x05, 0x06, 0x07, 0x08},
					}.Write(bconn.Writer)
					require.NoError(t, err)
				}

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}()

			c := &Client{
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
			require.NoError(t, err)

			err = c.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			recvDone := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				c.ReadFrames(func(trackID int, streamType StreamType, payload []byte) {
					require.Equal(t, 0, trackID)
					require.Equal(t, StreamTypeRTCP, streamType)
					require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, payload)
					close(recvDone)
				})
			}()

			err = c.WritePacketRTP(0,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.NoError(t, err)

			<-recvDone
			c.Close()
			<-done

			err = c.WritePacketRTP(0,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.Error(t, err)
		})
	}
}

func TestClientPublishParallel(t *testing.T) {
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
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				req, err := readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Read(req.Header["Transport"])
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

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}()

			c := &Client{
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
			require.NoError(t, err)

			writerDone := make(chan struct{})
			defer func() { <-writerDone }()

			err = c.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer c.Close()

			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := c.WritePacketRTP(0,
						[]byte{0x01, 0x02, 0x03, 0x04})
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
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				req, err := readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Read(req.Header["Transport"])
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

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}()

			c := &Client{
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
			require.NoError(t, err)

			err = c.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer c.Close()

			err = c.WritePacketRTP(0,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.NoError(t, err)

			_, err = c.Pause()
			require.NoError(t, err)

			err = c.WritePacketRTP(0,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.Error(t, err)

			_, err = c.Record()
			require.NoError(t, err)

			err = c.WritePacketRTP(0,
				[]byte{0x01, 0x02, 0x03, 0x04})
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
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				req, err := readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Read(req.Header["Transport"])
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

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}()

			c := &Client{
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
			require.NoError(t, err)

			err = c.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			writerDone := make(chan struct{})
			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := c.WritePacketRTP(0,
						[]byte{0x01, 0x02, 0x03, 0x04})
					if err != nil {
						return
					}
				}
			}()

			time.Sleep(1 * time.Second)

			_, err = c.Pause()
			require.NoError(t, err)
			<-writerDone

			c.Close()
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
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		req, err := readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		err = base.Response{
			StatusCode: base.StatusUnsupportedTransport,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Read(req.Header["Transport"])
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

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Write(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Record, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		var f base.InterleavedFrame
		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, 0, f.Channel)
		require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, f.Payload)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
	require.NoError(t, err)

	c := Client{}

	err = c.DialPublish("rtsp://localhost:8554/teststream",
		Tracks{track})
	require.NoError(t, err)
	defer c.Close()

	err = c.WritePacketRTP(0,
		[]byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
}

func TestClientPublishRTCPReport(t *testing.T) {
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
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		req, err := readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Read(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Write(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Record, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		rr := rtcpreceiver.New(nil, 90000)

		var f base.InterleavedFrame
		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, 0, f.Channel)
		rr.ProcessPacketRTP(time.Now(), f.Payload)

		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, 1, f.Channel)
		pkt, err := rtcp.Unmarshal(f.Payload)
		require.NoError(t, err)
		sr, ok := pkt[0].(*rtcp.SenderReport)
		require.True(t, ok)
		require.Equal(t, &rtcp.SenderReport{
			SSRC:        753621,
			NTPTime:     sr.NTPTime,
			RTPTime:     sr.RTPTime,
			PacketCount: 1,
			OctetCount:  4,
		}, sr)
		rr.ProcessPacketRTCP(time.Now(), f.Payload)

		err = base.InterleavedFrame{
			Channel: 1,
			Payload: rr.Report(time.Now()),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, 0, f.Channel)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
	}()

	c := &Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
		senderReportPeriod: 1 * time.Second,
	}

	track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
	require.NoError(t, err)

	err = c.DialPublish("rtsp://localhost:8554/teststream",
		Tracks{track})
	require.NoError(t, err)
	defer c.Close()

	byts, _ := (&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      54352,
			SSRC:           753621,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}).Marshal()
	err = c.WritePacketRTP(0, byts)
	require.NoError(t, err)

	time.Sleep(1300 * time.Millisecond)

	err = c.WritePacketRTP(0, byts)
	require.NoError(t, err)
}
