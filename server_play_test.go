package gortsplib

import (
	"bytes"
	"crypto/tls"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/conn"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/headers"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/aler9/gortsplib/v2/pkg/sdp"
	"github.com/aler9/gortsplib/v2/pkg/url"
)

func multicastCapableIP(t *testing.T) string {
	intfs, err := net.Interfaces()
	require.NoError(t, err)

	for _, intf := range intfs {
		if (intf.Flags & net.FlagMulticast) != 0 {
			addrs, err := intf.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					return v.IP.String()
				case *net.IPAddr:
					return v.IP.String()
				}
			}
		}
	}

	t.Errorf("unable to find a multicast IP")
	return ""
}

func TestServerPlaySetupPath(t *testing.T) {
	for _, ca := range []struct {
		name string
		url  string
		path string
	}{
		{
			"normal",
			"rtsp://localhost:8554/teststream/mediaID=2",
			"teststream",
		},
		{
			"with query",
			"rtsp://localhost:8554/teststream?testing=123/mediaID=4",
			"teststream",
		},
		{
			// this is needed to support reading mpegts with ffmpeg
			"without media id",
			"rtsp://localhost:8554/teststream/",
			"teststream",
		},
		{
			"subpath",
			"rtsp://localhost:8554/test/stream/mediaID=0",
			"test/stream",
		},
		{
			"subpath without media id",
			"rtsp://localhost:8554/test/stream/",
			"test/stream",
		},
		{
			"subpath with query",
			"rtsp://localhost:8554/test/stream?testing=123/mediaID=4",
			"test/stream",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			medi := testH264Media.Clone()
			stream := NewServerStream(media.Medias{medi, medi, medi, medi, medi})
			defer stream.Close()

			s := &Server{
				Handler: &testServerHandler{
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						require.Equal(t, ca.path, ctx.Path)
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
				},
				RTSPAddress: "localhost:8554",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			th := &headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL(ca.url),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": th.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerPlaySetupErrors(t *testing.T) {
	for _, ca := range []string{
		"different paths",
		"double setup",
		"closed stream",
	} {
		t.Run(ca, func(t *testing.T) {
			nconnClosed := make(chan struct{})

			stream := NewServerStream(media.Medias{testH264Media.Clone()})
			if ca == "closed stream" {
				stream.Close()
			} else {
				defer stream.Close()
			}

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						switch ca {
						case "different paths":
							require.EqualError(t, ctx.Error, "can't setup medias with different paths")

						case "double setup":
							require.EqualError(t, ctx.Error, "media has already been setup")

						case "closed stream":
							require.EqualError(t, ctx.Error, "stream is closed")
						}
						close(nconnClosed)
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
				},
				RTSPAddress: "localhost:8554",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			th := &headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": th.Marshal(),
				},
			})

			switch ca {
			case "different paths":
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				var sx headers.Session
				err = sx.Unmarshal(res.Header["Session"])
				require.NoError(t, err)
				th.InterleavedIDs = &[2]int{2, 3}

				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/test12stream/mediaID=1"),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"2"},
						"Transport": th.Marshal(),
						"Session":   base.HeaderValue{sx.Session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)

			case "double setup":
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				var sx headers.Session
				err = sx.Unmarshal(res.Header["Session"])
				require.NoError(t, err)
				th.InterleavedIDs = &[2]int{2, 3}

				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"2"},
						"Transport": th.Marshal(),
						"Session":   base.HeaderValue{sx.Session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)

			case "closed stream":
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)
			}

			<-nconnClosed
		})
	}
}

func TestServerPlaySetupErrorSameUDPPortsAndIP(t *testing.T) {
	stream := NewServerStream(media.Medias{testH264Media.Clone()})
	defer stream.Close()
	first := int32(1)
	errorRecv := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				if atomic.SwapInt32(&first, 0) == 1 {
					require.EqualError(t, ctx.Error,
						"UDP ports 35466 and 35467 are already assigned to another reader with the same IP")
					close(errorRecv)
				}
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
		RTSPAddress:    "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	for i := 0; i < 2; i++ {
		nconn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, err := writeReqReadRes(conn, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"1"},
				"Transport": inTH.Marshal(),
			},
		})
		require.NoError(t, err)

		if i == 0 {
			require.Equal(t, base.StatusOK, res.StatusCode)
		} else {
			require.Equal(t, base.StatusBadRequest, res.StatusCode)
		}
	}

	<-errorRecv
}

func TestServerPlay(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
		"tls",
		"multicast",
	} {
		t.Run(transport, func(t *testing.T) {
			nconnOpened := make(chan struct{})
			nconnClosed := make(chan struct{})
			sessionOpened := make(chan struct{})
			sessionClosed := make(chan struct{})
			framesReceived := make(chan struct{})

			stream := NewServerStream(media.Medias{testH264Media.Clone()})
			defer stream.Close()

			counter := uint64(0)

			listenIP := multicastCapableIP(t)

			s := &Server{
				Handler: &testServerHandler{
					onConnOpen: func(ctx *ServerHandlerOnConnOpenCtx) {
						close(nconnOpened)
					},
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						close(nconnClosed)
					},
					onSessionOpen: func(ctx *ServerHandlerOnSessionOpenCtx) {
						close(sessionOpened)
					},
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						// send RTCP packets directly to the session.
						// these are sent after the response, only if onPlay returns StatusOK.
						if transport != "multicast" {
							ctx.Session.WritePacketRTCP(stream.Medias()[0], &testRTCPPacket)
						}

						ctx.Session.OnPacketRTCPAny(func(medi *media.Media, pkt rtcp.Packet) {
							// ignore multicast loopback
							if transport == "multicast" && atomic.AddUint64(&counter, 1) <= 1 {
								return
							}

							require.Equal(t, stream.Medias()[0], medi)
							require.Equal(t, &testRTCPPacket, pkt)
							close(framesReceived)
						})

						// the session is added to the stream only after onPlay returns
						// with StatusOK; therefore we must wait before calling
						// ServerStream.WritePacket*()
						go func() {
							time.Sleep(1 * time.Second)
							stream.WritePacketRTCP(stream.Medias()[0], &testRTCPPacket)
							stream.WritePacketRTP(stream.Medias()[0], &testRTPPacket)
						}()

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onGetParameter: func(ctx *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress: listenIP + ":8554",
			}

			switch transport {
			case "udp":
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"

			case "multicast":
				s.MulticastIPRange = "224.1.0.0/16"
				s.MulticastRTPPort = 8000
				s.MulticastRTCPPort = 8001

			case "tls":
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", listenIP+":8554")
			require.NoError(t, err)

			nconn = func() net.Conn {
				if transport == "tls" {
					return tls.Client(nconn, &tls.Config{InsecureSkipVerify: true})
				}
				return nconn
			}()
			conn := conn.NewConn(nconn)

			<-nconnOpened

			inTH := &headers.Transport{
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
			}

			switch transport {
			case "udp":
				v := headers.TransportDeliveryUnicast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}

			case "multicast":
				v := headers.TransportDeliveryMulticast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP

			default:
				v := headers.TransportDeliveryUnicast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{4, 5}
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream/mediaID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var th headers.Transport
			err = th.Unmarshal(res.Header["Transport"])
			require.NoError(t, err)

			var l1 net.PacketConn
			var l2 net.PacketConn

			switch transport {
			case "udp":
				require.Equal(t, headers.TransportProtocolUDP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)

				l1, err = net.ListenPacket("udp", listenIP+":35466")
				require.NoError(t, err)
				defer l1.Close()

				l2, err = net.ListenPacket("udp", listenIP+":35467")
				require.NoError(t, err)
				defer l2.Close()

			case "multicast":
				require.Equal(t, headers.TransportProtocolUDP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryMulticast, *th.Delivery)

				l1, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[0]), 10))
				require.NoError(t, err)
				defer l1.Close()

				p := ipv4.NewPacketConn(l1)

				intfs, err := net.Interfaces()
				require.NoError(t, err)

				for _, intf := range intfs {
					err := p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
					require.NoError(t, err)
				}

				l2, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[1]), 10))
				require.NoError(t, err)
				defer l2.Close()

				p = ipv4.NewPacketConn(l2)

				intfs, err = net.Interfaces()
				require.NoError(t, err)

				for _, intf := range intfs {
					err := p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
					require.NoError(t, err)
				}

			default:
				require.Equal(t, headers.TransportProtocolTCP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)
			}

			<-sessionOpened

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			// server -> client (direct)
			switch transport {
			case "udp":
				buf := make([]byte, 2048)
				n, _, err := l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTCPPacketMarshaled, buf[:n])

			case "tcp", "tls":
				f, err := conn.ReadInterleavedFrame()
				require.NoError(t, err)

				switch f.Channel {
				case 4:
					require.Equal(t, testRTPPacketMarshaled, f.Payload)

				case 5:
					require.Equal(t, testRTCPPacketMarshaled, f.Payload)

				default:
					t.Errorf("should not happen")
				}
			}

			// server -> client (through stream)
			if transport == "udp" || transport == "multicast" {
				buf := make([]byte, 2048)
				n, _, err := l1.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTPPacketMarshaled, buf[:n])

				buf = make([]byte, 2048)
				n, _, err = l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTCPPacketMarshaled, buf[:n])
			} else {
				for i := 0; i < 2; i++ {
					f, err := conn.ReadInterleavedFrame()
					require.NoError(t, err)

					switch f.Channel {
					case 4:
						require.Equal(t, testRTPPacketMarshaled, f.Payload)

					case 5:
						require.Equal(t, testRTCPPacketMarshaled, f.Payload)

					default:
						t.Errorf("should not happen")
					}
				}
			}

			// client -> server (RTCP)
			switch transport {
			case "udp":
				l2.WriteTo(testRTCPPacketMarshaled, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
				<-framesReceived

			case "multicast":
				l2.WriteTo(testRTCPPacketMarshaled, &net.UDPAddr{
					IP:   *th.Destination,
					Port: th.Ports[1],
				})
				<-framesReceived

			default:
				err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 5,
					Payload: testRTCPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err)
				<-framesReceived
			}

			if transport == "udp" || transport == "multicast" {
				// ping with OPTIONS
				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Options,
					URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"4"},
						"Session": base.HeaderValue{sx.Session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				// ping with GET_PARAMETER
				res, err = writeReqReadRes(conn, base.Request{
					Method: base.GetParameter,
					URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"5"},
						"Session": base.HeaderValue{sx.Session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)
			}

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Teardown,
				URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"6"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionClosed

			nconn.Close()
			<-nconnClosed
		})
	}
}

func TestServerPlayDecodeErrors(t *testing.T) {
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

			stream := NewServerStream(media.Medias{testH264Media.Clone()})
			defer stream.Close()

			s := &Server{
				Handler: &testServerHandler{
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onDecodeError: func(ctx *ServerHandlerOnDecodeErrorCtx) {
						switch {
						case ca.proto == "udp" && ca.name == "rtcp invalid":
							require.EqualError(t, ctx.Error, "rtcp: packet too short")

						case ca.proto == "udp" && ca.name == "rtcp too big":
							require.EqualError(t, ctx.Error, "RTCP packet is too big to be read with UDP")

						case ca.proto == "tcp" && ca.name == "rtcp invalid":
							require.EqualError(t, ctx.Error, "rtcp: packet too short")

						case ca.proto == "tcp" && ca.name == "rtcp too big":
							require.EqualError(t, ctx.Error, "RTCP packet size (2000) is greater than maximum allowed (1472)")
						}
						close(errorRecv)
					},
				},
				UDPRTPAddress:  "127.0.0.1:8000",
				UDPRTCPAddress: "127.0.0.1:8001",
				RTSPAddress:    "localhost:8554",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			inTH := &headers.Transport{
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
			}

			if ca.proto == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var resTH headers.Transport
			err = resTH.Unmarshal(res.Header["Transport"])
			require.NoError(t, err)

			var l1 net.PacketConn
			var l2 net.PacketConn

			if ca.proto == "udp" {
				l1, err = net.ListenPacket("udp", "127.0.0.1:35466")
				require.NoError(t, err)
				defer l1.Close()

				l2, err = net.ListenPacket("udp", "127.0.0.1:35467")
				require.NoError(t, err)
				defer l2.Close()
			}

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			switch { //nolint:dupl
			case ca.proto == "udp" && ca.name == "rtcp invalid":
				l2.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[1],
				})

			case ca.proto == "udp" && ca.name == "rtcp too big":
				l2.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[1],
				})

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

			<-errorRecv
		})
	}
}

func TestServerPlayRTCPReport(t *testing.T) {
	for _, ca := range []string{"udp", "tcp"} {
		t.Run(ca, func(t *testing.T) {
			stream := NewServerStream(media.Medias{testH264Media.Clone()})
			defer stream.Close()

			s := &Server{
				Handler: &testServerHandler{
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				senderReportPeriod: 1 * time.Second,
				RTSPAddress:        "localhost:8554",
				UDPRTPAddress:      "127.0.0.1:8000",
				UDPRTCPAddress:     "127.0.0.1:8001",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			inTH := &headers.Transport{
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
			}

			if ca == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var l1 net.PacketConn
			var l2 net.PacketConn
			if ca == "udp" {
				l1, err = net.ListenPacket("udp", "localhost:35466")
				require.NoError(t, err)
				defer l1.Close()

				l2, err = net.ListenPacket("udp", "localhost:35467")
				require.NoError(t, err)
				defer l2.Close()
			}

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			for i := 0; i < 2; i++ {
				stream.WritePacketRTP(stream.Medias()[0], &rtp.Packet{
					Header: rtp.Header{
						Version:     2,
						PayloadType: 96,
						SSRC:        0x38F27A2F,
					},
					Payload: []byte{0x05}, // IDR
				})
			}

			var buf []byte

			if ca == "udp" {
				buf = make([]byte, 2048)
				var n int
				n, _, err = l2.ReadFrom(buf)
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
				NTPTime:     packets[0].(*rtcp.SenderReport).NTPTime,
				RTPTime:     packets[0].(*rtcp.SenderReport).RTPTime,
				PacketCount: 2,
				OctetCount:  2,
			}, packets[0])

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Teardown,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerPlayVLCMulticast(t *testing.T) {
	stream := NewServerStream(media.Medias{testH264Media.Clone()})
	defer stream.Close()

	listenIP := multicastCapableIP(t)

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(ctx *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
		},
		RTSPAddress:       listenIP + ":8554",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8000,
		MulticastRTCPPort: 8001,
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", listenIP+":8554")
	require.NoError(t, err)
	conn := conn.NewConn(nconn)
	defer nconn.Close()

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Describe,
		URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream?vlcmulticast"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var desc sdp.SessionDescription
	err = desc.Unmarshal(res.Body)
	require.NoError(t, err)

	require.Equal(t, "224.1.0.0", desc.ConnectionInformation.Address.Address)
}

func TestServerPlayTCPResponseBeforeFrames(t *testing.T) {
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	stream := NewServerStream(media.Medias{testH264Media.Clone()})
	defer stream.Close()

	s := &Server{
		RTSPAddress: "localhost:8554",
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					defer close(writerDone)

					stream.WritePacketRTP(stream.Medias()[0], &testRTPPacket)

					t := time.NewTicker(50 * time.Millisecond)
					defer t.Stop()

					for {
						select {
						case <-t.C:
							stream.WritePacketRTP(stream.Medias()[0], &testRTPPacket)
						case <-writerTerminate:
							return
						}
					}
				}()

				time.Sleep(50 * time.Millisecond)

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	_, err = conn.ReadInterleavedFrame()
	require.NoError(t, err)
}

func TestServerPlayPlayPlay(t *testing.T) {
	stream := NewServerStream(media.Medias{testH264Media.Clone()})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
		RTSPAddress:    "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolUDP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				ClientPorts: &[2]int{30450, 30451},
			}.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerPlayPlayPausePlay(t *testing.T) {
	writerStarted := false
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	stream := NewServerStream(media.Medias{testH264Media.Clone()})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				if !writerStarted {
					writerStarted = true
					go func() {
						defer close(writerDone)

						t := time.NewTicker(50 * time.Millisecond)
						defer t.Stop()

						for {
							select {
							case <-t.C:
								stream.WritePacketRTP(stream.Medias()[0], &testRTPPacket)
							case <-writerTerminate:
								return
							}
						}
					}()
				}

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Pause,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerPlayPlayPausePause(t *testing.T) {
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	stream := NewServerStream(media.Medias{testH264Media.Clone()})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					defer close(writerDone)

					t := time.NewTicker(50 * time.Millisecond)
					defer t.Stop()

					for {
						select {
						case <-t.C:
							stream.WritePacketRTP(stream.Medias()[0], &testRTPPacket)
						case <-writerTerminate:
							return
						}
					}
				}()

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = conn.WriteRequest(&base.Request{
		Method: base.Pause,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)

	res, err = conn.ReadResponseIgnoreFrames()
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = conn.WriteRequest(&base.Request{
		Method: base.Pause,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)

	res, err = conn.ReadResponseIgnoreFrames()
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerPlayTimeout(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"multicast",
		// there's no timeout when reading with TCP
	} {
		t.Run(transport, func(t *testing.T) {
			sessionClosed := make(chan struct{})

			stream := NewServerStream(media.Medias{testH264Media.Clone()})
			defer stream.Close()

			s := &Server{
				Handler: &testServerHandler{
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				ReadTimeout:    1 * time.Second,
				sessionTimeout: 1 * time.Second,
				RTSPAddress:    "localhost:8554",
			}

			switch transport {
			case "udp":
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"

			case "multicast":
				s.MulticastIPRange = "224.1.0.0/16"
				s.MulticastRTPPort = 8000
				s.MulticastRTCPPort = 8001
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			inTH := &headers.Transport{
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
			}

			switch transport {
			case "udp":
				v := headers.TransportDeliveryUnicast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}

			case "multicast":
				v := headers.TransportDeliveryMulticast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionClosed
		})
	}
}

func TestServerPlayWithoutTeardown(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			nconnClosed := make(chan struct{})
			sessionClosed := make(chan struct{})

			stream := NewServerStream(media.Medias{testH264Media.Clone()})
			defer stream.Close()

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						close(nconnClosed)
					},
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				ReadTimeout:    1 * time.Second,
				sessionTimeout: 1 * time.Second,
				RTSPAddress:    "localhost:8554",
			}

			if transport == "udp" {
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			inTH := &headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
			}

			if transport == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			nconn.Close()

			<-sessionClosed
			<-nconnClosed
		})
	}
}

func TestServerPlayUDPChangeConn(t *testing.T) {
	stream := NewServerStream(media.Medias{testH264Media.Clone()})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onGetParameter: func(ctx *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
		RTSPAddress:    "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	sxID := ""

	func() {
		nconn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, err := writeReqReadRes(conn, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"1"},
				"Transport": inTH.Marshal(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		var sx headers.Session
		err = sx.Unmarshal(res.Header["Session"])
		require.NoError(t, err)

		res, err = writeReqReadRes(conn, base.Request{
			Method: base.Play,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"2"},
				"Session": base.HeaderValue{sx.Session},
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		sxID = sx.Session
	}()

	func() {
		nconn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		res, err := writeReqReadRes(conn, base.Request{
			Method: base.GetParameter,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"1"},
				"Session": base.HeaderValue{sxID},
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)
	}()
}

func TestServerPlayPartialMedias(t *testing.T) {
	stream := NewServerStream(media.Medias{testH264Media.Clone(), testH264Media.Clone()})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					time.Sleep(1 * time.Second)
					stream.WritePacketRTP(stream.Medias()[0], &testRTPPacket)
					stream.WritePacketRTP(stream.Medias()[1], &testRTPPacket)
				}()

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	inTH := &headers.Transport{
		Delivery: func() *headers.TransportDelivery {
			v := headers.TransportDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModePlay
			return &v
		}(),
		Protocol:       headers.TransportProtocolTCP,
		InterleavedIDs: &[2]int{4, 5},
	}

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=1"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"1"},
			"Transport": inTH.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	f, err := conn.ReadInterleavedFrame()
	require.NoError(t, err)
	require.Equal(t, 4, f.Channel)
	require.Equal(t, testRTPPacketMarshaled, f.Payload)
}

func TestServerPlayAdditionalInfos(t *testing.T) {
	getInfos := func() (*headers.RTPInfo, []*uint32) {
		nconn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		ssrcs := make([]*uint32, 2)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		}

		res, err := writeReqReadRes(conn, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"1"},
				"Transport": inTH.Marshal(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		var th headers.Transport
		err = th.Unmarshal(res.Header["Transport"])
		require.NoError(t, err)
		ssrcs[0] = th.SSRC

		inTH = &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{2, 3},
		}

		var sx headers.Session
		err = sx.Unmarshal(res.Header["Session"])
		require.NoError(t, err)

		res, err = writeReqReadRes(conn, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/mediaID=1"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"2"},
				"Transport": inTH.Marshal(),
				"Session":   base.HeaderValue{sx.Session},
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		th = headers.Transport{}
		err = th.Unmarshal(res.Header["Transport"])
		require.NoError(t, err)
		ssrcs[1] = th.SSRC

		res, err = writeReqReadRes(conn, base.Request{
			Method: base.Play,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"3"},
				"Session": base.HeaderValue{sx.Session},
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		var ri headers.RTPInfo
		err = ri.Unmarshal(res.Header["RTP-Info"])
		require.NoError(t, err)

		return &ri, ssrcs
	}

	forma := &format.Generic{
		PayloadTyp: 96,
		RTPMap:     "private/90000",
	}
	err := forma.Init()
	require.NoError(t, err)

	medi := &media.Media{
		Type:    "application",
		Formats: []format.Format{forma},
	}

	stream := NewServerStream(media.Medias{medi.Clone(), medi.Clone()})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	stream.WritePacketRTP(stream.Medias()[0], &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 556,
			Timestamp:      984512368,
			SSRC:           96342362,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	})

	rtpInfo, ssrcs := getInfos()
	require.Equal(t, &headers.RTPInfo{
		&headers.RTPInfoEntry{
			URL: (&url.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   "/teststream/mediaID=0",
			}).String(),
			SequenceNumber: func() *uint16 {
				v := uint16(557)
				return &v
			}(),
			Timestamp: (*rtpInfo)[0].Timestamp,
		},
	}, rtpInfo)
	require.Equal(t, []*uint32{
		func() *uint32 {
			v := uint32(96342362)
			return &v
		}(),
		nil,
	}, ssrcs)

	stream.WritePacketRTP(stream.Medias()[1], &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 87,
			Timestamp:      756436454,
			SSRC:           536474323,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	})

	rtpInfo, ssrcs = getInfos()
	require.Equal(t, &headers.RTPInfo{
		&headers.RTPInfoEntry{
			URL: (&url.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   "/teststream/mediaID=0",
			}).String(),
			SequenceNumber: func() *uint16 {
				v := uint16(557)
				return &v
			}(),
			Timestamp: (*rtpInfo)[0].Timestamp,
		},
		&headers.RTPInfoEntry{
			URL: (&url.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   "/teststream/mediaID=1",
			}).String(),
			SequenceNumber: func() *uint16 {
				v := uint16(88)
				return &v
			}(),
			Timestamp: (*rtpInfo)[1].Timestamp,
		},
	}, rtpInfo)
	require.Equal(t, []*uint32{
		func() *uint32 {
			v := uint32(96342362)
			return &v
		}(),
		func() *uint32 {
			v := uint32(536474323)
			return &v
		}(),
	}, ssrcs)
}
