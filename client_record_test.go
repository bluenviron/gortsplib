package gortsplib

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/conn"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
)

var testH264Media = &description.Media{
	Type: description.MediaTypeVideo,
	Formats: []format.Format{&format.H264{
		PayloadTyp: 96,
		SPS: []byte{
			0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
			0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
			0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
			0x20,
		},
		PPS: []byte{
			0x44, 0x01, 0xc0, 0x25, 0x2f, 0x05, 0x32, 0x40,
		},
		PacketizationMode: 1,
	}},
}

var testRTPPacket = rtp.Packet{
	Header: rtp.Header{
		Version:     2,
		PayloadType: 96,
		CSRC:        []uint32{},
		SSRC:        0x38F27A2F,
	},
	Payload: []byte{1, 2, 3, 4},
}

var testRTPPacketMarshaled = mustMarshalPacketRTP(&testRTPPacket)

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

var testRTCPPacketMarshaled = mustMarshalPacketRTCP(&testRTCPPacket)

func ntpTimeGoToRTCP(v time.Time) uint64 {
	s := uint64(v.UnixNano()) + 2208988800*1000000000
	return (s/1000000000)<<32 | (s % 1000000000)
}

func record(c *Client, ur string, medias []*description.Media, cb func(*description.Media, rtcp.Packet)) error {
	u, err := base.ParseURL(ur)
	if err != nil {
		return err
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	_, err = c.Announce(u, &description.Session{Medias: medias})
	if err != nil {
		c.Close()
		return err
	}

	err = c.SetupAll(u, medias)
	if err != nil {
		c.Close()
		return err
	}

	if cb != nil {
		c.OnPacketRTCPAny(cb)
	}

	_, err = c.Record()
	if err != nil {
		c.Close()
		return err
	}

	return nil
}

func readRequestIgnoreFrames(c *conn.Conn) (*base.Request, error) {
	for {
		what, err := c.Read()
		if err != nil {
			return nil, err
		}

		switch what := what.(type) {
		case *base.InterleavedFrame:
		case *base.Request:
			return what, nil
		case *base.Response:
			return nil, fmt.Errorf("unexpected response")
		}
	}
}

func TestClientRecordSerial(t *testing.T) {
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

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				var desc sdp.SessionDescription
				err = desc.Unmarshal(req.Body)
				require.NoError(t, err)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL(
					scheme+"://localhost:8554/teststream/"+relativeControlAttribute(desc.MediaDescriptions[0])), req.URL)

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
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
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
					f, err := conn.ReadInterleavedFrame()
					require.NoError(t, err)
					require.Equal(t, 0, f.Channel)

					var pkt rtp.Packet
					err = pkt.Unmarshal(f.Payload)
					require.NoError(t, err)
					require.Equal(t, testRTPPacket, pkt)
				}

				// server -> client (RTCP)
				if transport == "udp" {
					_, err := l2.WriteTo(testRTCPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[1],
					})
					require.NoError(t, err)
				} else {
					err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
						Channel: 1,
						Payload: testRTCPPacketMarshaled,
					}, make([]byte, 1024))
					require.NoError(t, err)
				}

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
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
			}

			medi := testH264Media
			medias := []*description.Media{medi}

			err = record(&c, scheme+"://localhost:8554/teststream", medias,
				func(medi *description.Media, pkt rtcp.Packet) {
					require.Equal(t, &testRTCPPacket, pkt)
					close(recvDone)
				})
			require.NoError(t, err)

			done := make(chan struct{})
			go func() {
				defer close(done)
				c.Wait() //nolint:errcheck
			}()

			err = c.WritePacketRTP(medi, &testRTPPacket)
			require.NoError(t, err)

			<-recvDone
			c.Close()
			<-done

			err = c.WritePacketRTP(medi, &testRTPPacket)
			require.Error(t, err)
		})
	}
}

func TestClientRecordParallel(t *testing.T) {
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

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(conn)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
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

			writerDone := make(chan struct{})
			defer func() { <-writerDone }()

			medi := testH264Media
			medias := []*description.Media{medi}

			err = record(&c, scheme+"://localhost:8554/teststream", medias, nil)
			require.NoError(t, err)
			defer c.Close()

			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := c.WritePacketRTP(medi, &testRTPPacket)
					if err != nil {
						return
					}
				}
			}()

			time.Sleep(1 * time.Second)
		})
	}
}

func TestClientRecordPauseSerial(t *testing.T) {
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

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(conn)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(conn)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
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

			medi := testH264Media
			medias := []*description.Media{medi}

			err = record(&c, "rtsp://localhost:8554/teststream", medias, nil)
			require.NoError(t, err)
			defer c.Close()

			err = c.WritePacketRTP(medi, &testRTPPacket)
			require.NoError(t, err)

			_, err = c.Pause()
			require.NoError(t, err)

			_, err = c.Record()
			require.NoError(t, err)

			err = c.WritePacketRTP(medi, &testRTPPacket)
			require.NoError(t, err)
		})
	}
}

func TestClientRecordPauseParallel(t *testing.T) {
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

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = readRequestIgnoreFrames(conn)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
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

			medi := testH264Media
			medias := []*description.Media{medi}

			err = record(&c, "rtsp://localhost:8554/teststream", medias, nil)
			require.NoError(t, err)

			writerDone := make(chan struct{})
			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := c.WritePacketRTP(medi, &testRTPPacket)
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

func TestClientRecordAutomaticProtocol(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	recv := make(chan struct{})

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusUnsupportedTransport,
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)
		require.Equal(t, headers.TransportProtocolTCP, inTH.Protocol)

		th := headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Record, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		f, err := conn.ReadInterleavedFrame()
		require.NoError(t, err)
		require.Equal(t, 0, f.Channel)
		var pkt rtp.Packet
		err = pkt.Unmarshal(f.Payload)
		require.NoError(t, err)
		require.Equal(t, testRTPPacket, pkt)

		close(recv)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	c := Client{}

	medi := testH264Media
	medias := []*description.Media{medi}

	err = record(&c, "rtsp://localhost:8554/teststream", medias, nil)
	require.NoError(t, err)
	defer c.Close()

	err = c.WritePacketRTP(medi, &testRTPPacket)
	require.NoError(t, err)

	<-recv
}

func TestClientRecordDecodeErrors(t *testing.T) {
	for _, ca := range []struct {
		proto string
		name  string
	}{
		{"udp", "rtcp invalid"},
		{"udp", "rtcp too big"},
		{"tcp", "rtcp invalid"},
		{"tcp", "rtcp too big"},
	} {
		t.Run(ca.proto+" "+ca.name, func(t *testing.T) {
			errorRecv := make(chan struct{})

			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				if ca.proto == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ClientPorts = inTH.ClientPorts
					th.ServerPorts = &[2]int{34556, 34557}
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				var l1 net.PacketConn
				var l2 net.PacketConn

				if ca.proto == "udp" {
					l1, err = net.ListenPacket("udp", "127.0.0.1:34556")
					require.NoError(t, err)
					defer l1.Close()

					l2, err = net.ListenPacket("udp", "127.0.0.1:34557")
					require.NoError(t, err)
					defer l2.Close()
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				switch { //nolint:dupl
				case ca.proto == "udp" && ca.name == "rtcp invalid":
					_, err := l2.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[1],
					})
					require.NoError(t, err)

				case ca.proto == "udp" && ca.name == "rtcp too big":
					_, err := l2.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[1],
					})
					require.NoError(t, err)

				case ca.proto == "tcp" && ca.name == "rtcp invalid":
					err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
						Channel: 1,
						Payload: []byte{0x01, 0x02},
					}, make([]byte, 2048))
					require.NoError(t, err)

				case ca.proto == "tcp" && ca.name == "rtcp too big":
					err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
						Channel: 1,
						Payload: bytes.Repeat([]byte{0x01, 0x02}, 2000/2),
					}, make([]byte, 2048))
					require.NoError(t, err)
				}

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			c := Client{
				Transport: func() *Transport {
					if ca.proto == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
				OnDecodeError: func(err error) {
					switch {
					case ca.name == "rtcp invalid":
						require.EqualError(t, err, "rtcp: packet too short")

					case ca.proto == "udp" && ca.name == "rtcp too big":
						require.EqualError(t, err, "RTCP packet is too big to be read with UDP")

					case ca.proto == "tcp" && ca.name == "rtcp too big":
						require.EqualError(t, err, "RTCP packet size (2000) is greater than maximum allowed (1472)")
					}
					close(errorRecv)
				},
			}

			medias := []*description.Media{testH264Media}

			err = record(&c, "rtsp://localhost:8554/stream", medias, nil)
			require.NoError(t, err)
			defer c.Close()

			<-errorRecv
		})
	}
}

func TestClientRecordRTCPReport(t *testing.T) {
	for _, ca := range []string{"udp", "tcp"} {
		t.Run(ca, func(t *testing.T) {
			reportReceived := make(chan struct{})

			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				if ca == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ClientPorts = inTH.ClientPorts
					th.ServerPorts = &[2]int{34556, 34557}
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				l1, err := net.ListenPacket("udp", "localhost:34556")
				require.NoError(t, err)
				defer l1.Close()

				l2, err := net.ListenPacket("udp", "localhost:34557")
				require.NoError(t, err)
				defer l2.Close()

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				var buf []byte

				if ca == "udp" {
					buf = make([]byte, 2048)
					n, _, err := l2.ReadFrom(buf)
					require.NoError(t, err)
					buf = buf[:n]
				} else {
					for i := 0; i < 2; i++ {
						_, err := conn.ReadInterleavedFrame()
						require.NoError(t, err)
					}

					f, err := conn.ReadInterleavedFrame()
					require.NoError(t, err)
					require.Equal(t, 1, f.Channel)
					buf = f.Payload
				}

				packets, err := rtcp.Unmarshal(buf)
				require.NoError(t, err)
				require.Equal(t, &rtcp.SenderReport{
					SSRC:        0x38F27A2F,
					NTPTime:     ntpTimeGoToRTCP(time.Date(1996, 2, 13, 14, 33, 5, 0, time.UTC)),
					RTPTime:     1300000 + 60*90000,
					PacketCount: 1,
					OctetCount:  1,
				}, packets[0])

				close(reportReceived)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			var curTime time.Time
			var curTimeMutex sync.Mutex

			c := Client{
				Transport: func() *Transport {
					if ca == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
				timeNow: func() time.Time {
					curTimeMutex.Lock()
					defer curTimeMutex.Unlock()
					return curTime
				},
				senderReportPeriod: 100 * time.Millisecond,
			}

			medi := testH264Media
			medias := []*description.Media{medi}

			err = record(&c, "rtsp://localhost:8554/teststream", medias, nil)
			require.NoError(t, err)
			defer c.Close()

			curTimeMutex.Lock()
			curTime = time.Date(2013, 6, 10, 1, 0, 0, 0, time.UTC)
			curTimeMutex.Unlock()

			err = c.WritePacketRTPWithNTP(
				medi,
				&rtp.Packet{
					Header: rtp.Header{
						Version:     2,
						PayloadType: 96,
						SSRC:        0x38F27A2F,
						Timestamp:   1300000,
					},
					Payload: []byte{0x05}, // IDR
				},
				time.Date(1996, 2, 13, 14, 32, 5, 0, time.UTC))
			require.NoError(t, err)

			curTimeMutex.Lock()
			curTime = time.Date(2013, 6, 10, 1, 1, 0, 0, time.UTC)
			curTimeMutex.Unlock()

			<-reportReceived
		})
	}
}

func TestClientRecordIgnoreTCPRTPPackets(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Record, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err)

		err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 1,
			Payload: testRTCPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	rtcpReceived := make(chan struct{})

	c := Client{
		Transport: transportPtr(TransportTCP),
	}

	medias := []*description.Media{testH264Media}

	err = record(&c, "rtsp://localhost:8554/teststream", medias,
		func(medi *description.Media, pkt rtcp.Packet) {
			close(rtcpReceived)
		})
	require.NoError(t, err)
	defer c.Close()

	<-rtcpReceived
}
