package gortsplib

import (
	"bytes"
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/conn"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
)

func uintPtr(v uint) *uint {
	return &v
}

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

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

func mediaURL(t *testing.T, baseURL *base.URL, media *description.Media) *base.URL {
	u, err := media.URL(baseURL)
	require.NoError(t, err)
	return u
}

func doDescribe(t *testing.T, conn *conn.Conn) *description.Session {
	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Describe,
		URL:    mustParseURL("rtsp://localhost:8554/teststream?param=value"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var desc sdp.SessionDescription
	err = desc.Unmarshal(res.Body)
	require.NoError(t, err)

	var desc2 description.Session
	err = desc2.Unmarshal(&desc)
	require.NoError(t, err)

	desc2.BaseURL = mustParseURL(res.Header["Content-Base"][0])

	return &desc2
}

func doSetup(t *testing.T, conn *conn.Conn, u string,
	inTH *headers.Transport, session string,
) (*base.Response, *headers.Transport) {
	h := base.Header{
		"CSeq":      base.HeaderValue{"1"},
		"Transport": inTH.Marshal(),
	}

	if session != "" {
		h["Session"] = base.HeaderValue{session}
	}

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL(u),
		Header: h,
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var th headers.Transport
	err = th.Unmarshal(res.Header["Transport"])
	require.NoError(t, err)

	return res, &th
}

func doPlay(t *testing.T, conn *conn.Conn, u string, session string) *base.Response {
	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Play,
		URL:    mustParseURL(u),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"1"},
			"Session": base.HeaderValue{session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
	return res
}

func doPause(t *testing.T, conn *conn.Conn, u string, session string) {
	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Pause,
		URL:    mustParseURL(u),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"1"},
			"Session": base.HeaderValue{session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func doTeardown(t *testing.T, conn *conn.Conn, u string, session string) {
	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Teardown,
		URL:    mustParseURL(u),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"1"},
			"Session": base.HeaderValue{session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func readSession(t *testing.T, res *base.Response) string {
	var sx headers.Session
	err := sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)
	return sx.Session
}

func TestServerPlayPath(t *testing.T) {
	for _, ca := range []struct {
		name     string
		setupURL string
		playURL  string
		path     string
	}{
		{
			"standard",
			"rtsp://localhost:8554/teststream[control]",
			"rtsp://localhost:8554/teststream/",
			"/teststream",
		},
		{
			"no query, ffmpeg format",
			"rtsp://localhost:8554/teststream?testing=123[control]",
			"rtsp://localhost:8554/teststream/",
			"/teststream",
		},
		{
			"no query, gstreamer format",
			"rtsp://localhost:8554/teststream[control]?testing=123",
			"rtsp://localhost:8554/teststream/",
			"/teststream",
		},
		{
			// this is needed to support reading mpegts with ffmpeg
			"no control",
			"rtsp://localhost:8554/teststream/",
			"rtsp://localhost:8554/teststream/",
			"/teststream",
		},
		{
			"no control, query, ffmpeg",
			"rtsp://localhost:8554/teststream?testing=123/",
			"rtsp://localhost:8554/teststream/",
			"/teststream",
		},
		{
			"no control, query, gstreamer",
			"rtsp://localhost:8554/teststream/?testing=123",
			"rtsp://localhost:8554/teststream/",
			"/teststream",
		},
		{
			"subpath",
			"rtsp://localhost:8554/test/stream[control]",
			"rtsp://localhost:8554/test/stream/",
			"/test/stream",
		},
		{
			"subpath, no control",
			"rtsp://localhost:8554/test/stream/",
			"rtsp://localhost:8554/test/stream/",
			"/test/stream",
		},
		{
			"subpath with query, ffmpeg format",
			"rtsp://localhost:8554/test/stream?testing=123[control]",
			"rtsp://localhost:8554/test/stream",
			"/test/stream",
		},
		{
			"subpath with query, gstreamer format",
			"rtsp://localhost:8554/test/stream[control]?testing=123",
			"rtsp://localhost:8554/test/stream",
			"/test/stream",
		},
		{
			"no path, no slash",
			"rtsp://localhost:8554[control]",
			"rtsp://localhost:8554",
			"",
		},
		{
			"single slash",
			"rtsp://localhost:8554/[control]",
			"rtsp://localhost:8554//",
			"/",
		},
		{
			"no control, no path",
			"rtsp://localhost:8554/",
			"rtsp://localhost:8554/",
			"/",
		},
		{
			"no control, no path, no slash",
			"rtsp://localhost:8554",
			"rtsp://localhost:8554",
			"",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var stream *ServerStream

			s := &Server{
				Handler: &testServerHandler{
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						require.Equal(t, ca.path, ctx.Path)

						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						require.Equal(t, ca.path, ctx.Path)

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

			stream = &ServerStream{
				Server: s,
				Desc: &description.Session{
					Medias: []*description.Media{
						testH264Media,
						testH264Media,
						testH264Media,
						testH264Media,
					},
				},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			th := &headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
				Mode:           transportModePtr(headers.TransportModePlay),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, _ := doSetup(t, conn,
				strings.ReplaceAll(ca.setupURL, "[control]", "/"+desc.Medias[1].Control),
				th, "")

			session := readSession(t, res)

			doPlay(t, conn, ca.playURL, session)
		})
	}
}

func TestServerPlaySetupErrors(t *testing.T) {
	for _, ca := range []string{
		"invalid path",
		"closed stream",
		"different paths",
		"double setup",
		"different protocols",
	} {
		t.Run(ca, func(t *testing.T) {
			var stream *ServerStream
			nconnClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						switch ca {
						case "invalid path":
							require.EqualError(t, ctx.Error, "invalid SETUP path. "+
								"This typically happens when VLC fails a request, and then switches to an unsupported RTSP dialect")

						case "closed stream":
							require.EqualError(t, ctx.Error, "stream is closed")

						case "different paths":
							require.EqualError(t, ctx.Error, "can't setup medias with different paths")

						case "double setup":
							require.EqualError(t, ctx.Error, "media has already been setup")

						case "different protocols":
							require.EqualError(t, ctx.Error, "can't setup medias with different protocols")
						}
						close(nconnClosed)
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
				},
				RTSPAddress:    "localhost:8554",
				UDPRTPAddress:  "127.0.0.1:8000",
				UDPRTCPAddress: "127.0.0.1:8001",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)

			if ca == "closed stream" {
				stream.Close()
			} else {
				defer stream.Close()
			}

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			var desc *description.Session
			var th *headers.Transport
			var res *base.Response

			switch ca {
			case "invalid path":
				th = &headers.Transport{
					Protocol:    headers.TransportProtocolUDP,
					Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
					Mode:        transportModePtr(headers.TransportModePlay),
					ClientPorts: &[2]int{35466, 35467},
				}

				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL: &base.URL{
						Scheme: "rtsp",
						Path:   "/invalid",
					},
					Header: base.Header{
						"CSeq":      base.HeaderValue{"2"},
						"Transport": th.Marshal(),
					},
				})

				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)

			default:
				desc = doDescribe(t, conn)

				th = &headers.Transport{
					Protocol:    headers.TransportProtocolUDP,
					Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
					Mode:        transportModePtr(headers.TransportModePlay),
					ClientPorts: &[2]int{35466, 35467},
				}

				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"2"},
						"Transport": th.Marshal(),
					},
				})

				if ca != "closed stream" {
					require.NoError(t, err)
					require.Equal(t, base.StatusOK, res.StatusCode)
				}
			}

			switch ca {
			case "closed stream":
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)

			case "different paths":
				session := readSession(t, res)

				th.ClientPorts = &[2]int{35468, 35469}

				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/test12stream/" + desc.Medias[0].Control),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"3"},
						"Transport": th.Marshal(),
						"Session":   base.HeaderValue{session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)

			case "double setup":
				session := readSession(t, res)

				th.ClientPorts = &[2]int{35468, 35469}

				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"4"},
						"Transport": th.Marshal(),
						"Session":   base.HeaderValue{session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)

			case "different protocols":
				session := readSession(t, res)

				th.Protocol = headers.TransportProtocolTCP
				th.InterleavedIDs = &[2]int{0, 1}

				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"4"},
						"Transport": th.Marshal(),
						"Session":   base.HeaderValue{session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)
			}

			<-nconnClosed
		})
	}
}

func TestServerPlaySetupErrorSameUDPPortsAndIP(t *testing.T) {
	var stream *ServerStream
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
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
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

	stream = &ServerStream{
		Server: s,
		Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	for i := 0; i < 2; i++ {
		nconn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		inTH := &headers.Transport{
			Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
			Mode:        transportModePtr(headers.TransportModePlay),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		desc := doDescribe(t, conn)

		res, err := writeReqReadRes(conn, base.Request{
			Method: base.Setup,
			URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"2"},
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
		"multicast",
		"tcp",
		"tls",
	} {
		t.Run(transport, func(t *testing.T) {
			var stream *ServerStream
			nconnOpened := make(chan struct{})
			nconnClosed := make(chan struct{})
			sessionOpened := make(chan struct{})
			sessionClosed := make(chan struct{})
			framesReceived := make(chan struct{})
			counter := uint64(0)

			listenIP := multicastCapableIP(t)

			s := &Server{
				Handler: &testServerHandler{
					onConnOpen: func(_ *ServerHandlerOnConnOpenCtx) {
						close(nconnOpened)
					},
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						s := ctx.Conn.Stats()
						require.Greater(t, s.BytesSent, uint64(810))
						require.Less(t, s.BytesSent, uint64(1150))
						require.Greater(t, s.BytesReceived, uint64(440))
						require.Less(t, s.BytesReceived, uint64(660))

						close(nconnClosed)
					},
					onSessionOpen: func(_ *ServerHandlerOnSessionOpenCtx) {
						close(sessionOpened)
					},
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						if transport != "multicast" {
							s := ctx.Session.Stats()
							require.Greater(t, s.BytesSent, uint64(50))
							require.Less(t, s.BytesSent, uint64(60))
							require.Greater(t, s.BytesReceived, uint64(15))
							require.Less(t, s.BytesReceived, uint64(25))
						}

						close(sessionClosed)
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						require.NotNil(t, ctx.Conn.Session())

						switch transport {
						case "udp":
							v := TransportUDP
							require.Equal(t, &v, ctx.Session.SetuppedTransport())

						case "tcp", "tls":
							v := TransportTCP
							require.Equal(t, &v, ctx.Session.SetuppedTransport())

						case "multicast":
							v := TransportUDPMulticast
							require.Equal(t, &v, ctx.Session.SetuppedTransport())
						}

						require.Equal(t, "param=value", ctx.Session.SetuppedQuery())
						require.Equal(t, stream.Description().Medias, ctx.Session.SetuppedMedias())

						// send RTCP packets directly to the session.
						// these are sent after the response, only if onPlay returns StatusOK.
						if transport != "multicast" {
							err := ctx.Session.WritePacketRTCP(stream.Description().Medias[0], &testRTCPPacket)
							require.NoError(t, err)
						}

						ctx.Session.OnPacketRTCPAny(func(medi *description.Media, pkt rtcp.Packet) {
							// ignore multicast loopback
							if transport == "multicast" && atomic.AddUint64(&counter, 1) <= 1 {
								return
							}

							require.Equal(t, stream.Description().Medias[0], medi)
							require.Equal(t, &testRTCPPacket, pkt)
							close(framesReceived)
						})

						// the session is added to the stream only after onPlay returns
						// with StatusOK; therefore we must wait before calling
						// ServerStream.WritePacket*()
						go func() {
							time.Sleep(500 * time.Millisecond)
							err := stream.WritePacketRTCP(stream.Description().Medias[0], &testRTCPPacket)
							require.NoError(t, err)
							err = stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
							require.NoError(t, err)
						}()

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onGetParameter: func(_ *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
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

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

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

			desc := doDescribe(t, conn)

			inTH := &headers.Transport{
				Mode: transportModePtr(headers.TransportModePlay),
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
				inTH.InterleavedIDs = &[2]int{5, 6} // odd value
			}

			res, th := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

			var l1 net.PacketConn
			var l2 net.PacketConn

			switch transport { //nolint:dupl
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

				var intfs []net.Interface
				intfs, err = net.Interfaces()
				require.NoError(t, err)

				for _, intf := range intfs {
					err = p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
					require.NoError(t, err)
				}

				l2, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[1]), 10))
				require.NoError(t, err)
				defer l2.Close()

				p = ipv4.NewPacketConn(l2)

				intfs, err = net.Interfaces()
				require.NoError(t, err)

				for _, intf := range intfs {
					err = p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
					require.NoError(t, err)
				}

			default:
				require.Equal(t, headers.TransportProtocolTCP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)
			}

			<-sessionOpened

			session := readSession(t, res)

			doPlay(t, conn, "rtsp://"+listenIP+":8554/teststream", session)

			// server -> client (direct)
			switch transport {
			case "udp":
				buf := make([]byte, 2048)
				var n int
				n, _, err = l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTCPPacketMarshaled, buf[:n])

			case "tcp", "tls":
				var f *base.InterleavedFrame
				f, err = conn.ReadInterleavedFrame()
				require.NoError(t, err)

				switch f.Channel {
				case 5:
					require.Equal(t, testRTPPacketMarshaled, f.Payload)

				case 6:
					require.Equal(t, testRTCPPacketMarshaled, f.Payload)

				default:
					t.Errorf("should not happen")
				}
			}

			// server -> client (through stream)
			if transport == "udp" || transport == "multicast" {
				buf := make([]byte, 2048)
				var n int
				n, _, err = l1.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTPPacketMarshaled, buf[:n])

				buf = make([]byte, 2048)
				n, _, err = l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTCPPacketMarshaled, buf[:n])
			} else {
				for i := 0; i < 2; i++ {
					var f *base.InterleavedFrame
					f, err = conn.ReadInterleavedFrame()
					require.NoError(t, err)

					switch f.Channel {
					case 5:
						require.Equal(t, testRTPPacketMarshaled, f.Payload)

					case 6:
						require.Equal(t, testRTCPPacketMarshaled, f.Payload)

					default:
						t.Errorf("should not happen")
					}
				}
			}

			// client -> server (RTCP)
			switch transport {
			case "udp":
				_, err = l2.WriteTo(testRTCPPacketMarshaled, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
				require.NoError(t, err)
				<-framesReceived

			case "multicast":
				_, err = l2.WriteTo(testRTCPPacketMarshaled, &net.UDPAddr{
					IP:   *th.Destination,
					Port: th.Ports[1],
				})
				require.NoError(t, err)
				<-framesReceived

			default:
				err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 6,
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
						"Session": base.HeaderValue{session},
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
						"Session": base.HeaderValue{session},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)
			}

			doTeardown(t, conn, "rtsp://"+listenIP+":8554/teststream", session)

			<-sessionClosed

			nconn.Close()
			<-nconnClosed
		})
	}
}

func TestServerPlaySocketError(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"multicast",
		"tcp",
		"tls",
	} {
		t.Run(transport, func(t *testing.T) {
			var stream *ServerStream
			connClosed := make(chan struct{})
			writeDone := make(chan struct{})
			listenIP := multicastCapableIP(t)

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(_ *ServerHandlerOnConnCloseCtx) {
						close(connClosed)
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
						go func() {
							defer close(writeDone)

							t := time.NewTicker(50 * time.Millisecond)
							defer t.Stop()

							for range t.C {
								err := stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
								if err != nil {
									return
								}
							}
						}()

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

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)

			func() {
				nconn, err := net.Dial("tcp", listenIP+":8554")
				require.NoError(t, err)
				defer nconn.Close()

				nconn = func() net.Conn {
					if transport == "tls" {
						return tls.Client(nconn, &tls.Config{InsecureSkipVerify: true})
					}
					return nconn
				}()
				conn := conn.NewConn(nconn)

				desc := doDescribe(t, conn)

				inTH := &headers.Transport{
					Mode: transportModePtr(headers.TransportModePlay),
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
					inTH.InterleavedIDs = &[2]int{5, 6} // odd value
				}

				res, th := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

				var l1 net.PacketConn
				var l2 net.PacketConn

				switch transport { //nolint:dupl
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

					var intfs []net.Interface
					intfs, err = net.Interfaces()
					require.NoError(t, err)

					for _, intf := range intfs {
						err = p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
						require.NoError(t, err)
					}

					l2, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[1]), 10))
					require.NoError(t, err)
					defer l2.Close()

					p = ipv4.NewPacketConn(l2)

					intfs, err = net.Interfaces()
					require.NoError(t, err)

					for _, intf := range intfs {
						err = p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
						require.NoError(t, err)
					}

				default:
					require.Equal(t, headers.TransportProtocolTCP, th.Protocol)
					require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)
				}

				session := readSession(t, res)

				doPlay(t, conn, "rtsp://"+listenIP+":8554/teststream", session)
			}()

			<-connClosed

			stream.Close()
			<-writeDone
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
			var stream *ServerStream
			errorRecv := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
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

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			inTH := &headers.Transport{
				Mode:     transportModePtr(headers.TransportModePlay),
				Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
			}

			if ca.proto == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, resTH := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

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

			session := readSession(t, res)

			doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

			switch { //nolint:dupl
			case ca.proto == "udp" && ca.name == "rtcp invalid":
				_, err := l2.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[1],
				})
				require.NoError(t, err)

			case ca.proto == "udp" && ca.name == "rtcp too big":
				_, err := l2.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[1],
				})
				require.NoError(t, err)

			case ca.proto == "tcp" && ca.name == "rtcp invalid":
				err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 1,
					Payload: []byte{0x01, 0x02},
				}, make([]byte, 2048))
				require.NoError(t, err)

			case ca.proto == "tcp" && ca.name == "rtcp too big":
				err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
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
			var stream *ServerStream

			var curTime time.Time
			var curTimeMutex sync.Mutex

			s := &Server{
				Handler: &testServerHandler{
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress:    "localhost:8554",
				UDPRTPAddress:  "127.0.0.1:8000",
				UDPRTCPAddress: "127.0.0.1:8001",
				timeNow: func() time.Time {
					curTimeMutex.Lock()
					defer curTimeMutex.Unlock()
					return curTime
				},
				senderReportPeriod: 100 * time.Millisecond,
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			inTH := &headers.Transport{
				Mode:     transportModePtr(headers.TransportModePlay),
				Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
			}

			if ca == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

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

			session := readSession(t, res)

			doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

			curTimeMutex.Lock()
			curTime = time.Date(2014, 6, 7, 15, 0, 0, 0, time.UTC)
			curTimeMutex.Unlock()

			err = stream.WritePacketRTPWithNTP(
				stream.Description().Medias[0],
				&rtp.Packet{
					Header: rtp.Header{
						Version:     2,
						PayloadType: 96,
						SSRC:        0x38F27A2F,
						Timestamp:   240000,
					},
					Payload: []byte{0x05}, // IDR
				},
				time.Date(2017, 8, 10, 12, 22, 0, 0, time.UTC))
			require.NoError(t, err)

			curTimeMutex.Lock()
			curTime = time.Date(2014, 6, 7, 15, 0, 30, 0, time.UTC)
			curTimeMutex.Unlock()

			var buf []byte

			if ca == "udp" {
				buf = make([]byte, 2048)
				var n int
				n, _, err = l2.ReadFrom(buf)
				require.NoError(t, err)
				buf = buf[:n]
			} else {
				_, err = conn.ReadInterleavedFrame()
				require.NoError(t, err)

				var f *base.InterleavedFrame
				f, err = conn.ReadInterleavedFrame()
				require.NoError(t, err)
				require.Equal(t, 1, f.Channel)
				buf = f.Payload
			}

			packets, err := rtcp.Unmarshal(buf)
			require.NoError(t, err)
			require.Equal(t, &rtcp.SenderReport{
				SSRC:        0x38F27A2F,
				NTPTime:     ntpTimeGoToRTCP(time.Date(2017, 8, 10, 12, 22, 30, 0, time.UTC)),
				RTPTime:     240000 + 90000*30,
				PacketCount: 1,
				OctetCount:  1,
			}, packets[0])

			doTeardown(t, conn, "rtsp://localhost:8554/teststream", session)
		})
	}
}

func TestServerPlayVLCMulticast(t *testing.T) {
	var stream *ServerStream
	listenIP := multicastCapableIP(t)

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
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

	stream = &ServerStream{
		Server: s,
		Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

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
	var stream *ServerStream
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	s := &Server{
		RTSPAddress: "localhost:8554",
		Handler: &testServerHandler{
			onConnClose: func(_ *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					defer close(writerDone)

					err := stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
					require.NoError(t, err)

					ti := time.NewTicker(50 * time.Millisecond)
					defer ti.Stop()

					for {
						select {
						case <-ti.C:
							err := stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
							require.NoError(t, err)
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

	stream = &ServerStream{
		Server: s,
		Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:           transportModePtr(headers.TransportModePlay),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	session := readSession(t, res)

	doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

	_, err = conn.ReadInterleavedFrame()
	require.NoError(t, err)
}

func TestServerPlayPause(t *testing.T) {
	var stream *ServerStream
	writerStarted := false
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(_ *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
				if !writerStarted {
					writerStarted = true
					go func() {
						defer close(writerDone)

						ti := time.NewTicker(50 * time.Millisecond)
						defer ti.Stop()

						for {
							select {
							case <-ti.C:
								err := stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
								require.NoError(t, err)
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
			onPause: func(_ *ServerHandlerOnPauseCtx) (*base.Response, error) {
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

	stream = &ServerStream{
		Server: s,
		Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:           transportModePtr(headers.TransportModePlay),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	session := readSession(t, res)

	doPlay(t, conn, "rtsp://localhost:8554/teststream", session)
	doPause(t, conn, "rtsp://localhost:8554/teststream", session)
}

func TestServerPlayPlayPausePausePlay(t *testing.T) {
	for _, ca := range []string{"stream", "direct"} {
		t.Run(ca, func(t *testing.T) {
			var stream *ServerStream
			writerStarted := false
			writerDone := make(chan struct{})
			writerTerminate := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(_ *ServerHandlerOnConnCloseCtx) {
						close(writerTerminate)
						<-writerDone
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						if !writerStarted {
							writerStarted = true
							go func() {
								defer close(writerDone)

								ti := time.NewTicker(50 * time.Millisecond)
								defer ti.Stop()

								for {
									select {
									case <-ti.C:
										if ca == "stream" {
											err := stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
											require.NoError(t, err)
										} else {
											err := ctx.Session.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
											require.NoError(t, err)
										}

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
					onPause: func(_ *ServerHandlerOnPauseCtx) (*base.Response, error) {
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

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			inTH := &headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
				Mode:           transportModePtr(headers.TransportModePlay),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

			session := readSession(t, res)

			doPlay(t, conn, "rtsp://localhost:8554/teststream", session)
			doPlay(t, conn, "rtsp://localhost:8554/teststream", session)
			doPause(t, conn, "rtsp://localhost:8554/teststream", session)
			doPause(t, conn, "rtsp://localhost:8554/teststream", session)
			time.Sleep(500 * time.Millisecond)
			doPlay(t, conn, "rtsp://localhost:8554/teststream", session)
		})
	}
}

func TestServerPlayTimeout(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"multicast",
		// there's no timeout when reading with TCP
	} {
		t.Run(transport, func(t *testing.T) {
			var stream *ServerStream
			sessionClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onSessionClose: func(_ *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				ReadTimeout:       1 * time.Second,
				sessionTimeout:    1 * time.Second,
				RTSPAddress:       "localhost:8554",
				checkStreamPeriod: 500 * time.Millisecond,
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

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			inTH := &headers.Transport{
				Mode: transportModePtr(headers.TransportModePlay),
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

			res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

			session := readSession(t, res)

			doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

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
			var stream *ServerStream
			nconnClosed := make(chan struct{})
			sessionClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(_ *ServerHandlerOnConnCloseCtx) {
						close(nconnClosed)
					},
					onSessionClose: func(_ *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
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

			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			inTH := &headers.Transport{
				Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				Mode:     transportModePtr(headers.TransportModePlay),
			}

			if transport == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

			session := readSession(t, res)

			doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

			nconn.Close()

			<-sessionClosed
			<-nconnClosed
		})
	}
}

func TestServerPlayUDPChangeConn(t *testing.T) {
	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onGetParameter: func(_ *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
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

	stream = &ServerStream{
		Server: s,
		Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	sxID := ""

	func() {
		nconn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		desc := doDescribe(t, conn)

		inTH := &headers.Transport{
			Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
			Mode:        transportModePtr(headers.TransportModePlay),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

		session := readSession(t, res)

		doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

		sxID = session
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
				"CSeq":    base.HeaderValue{"4"},
				"Session": base.HeaderValue{sxID},
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)
	}()
}

func TestServerPlayPartialMedias(t *testing.T) {
	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					time.Sleep(500 * time.Millisecond)
					err := stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
					require.NoError(t, err)
					err = stream.WritePacketRTP(stream.Description().Medias[1], &testRTPPacket)
					require.NoError(t, err)
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

	stream = &ServerStream{
		Server: s,
		Desc:   &description.Session{Medias: []*description.Media{testH264Media, testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTH := &headers.Transport{
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:           transportModePtr(headers.TransportModePlay),
		Protocol:       headers.TransportProtocolTCP,
		InterleavedIDs: &[2]int{4, 5},
	}

	res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	session := readSession(t, res)

	doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

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

		desc := doDescribe(t, conn)

		inTH := &headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Mode:           transportModePtr(headers.TransportModePlay),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		}

		res, th := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

		ssrcs := make([]*uint32, 2)
		ssrcs[0] = th.SSRC

		inTH = &headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Mode:           transportModePtr(headers.TransportModePlay),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{2, 3},
		}

		session := readSession(t, res)

		_, th = doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[1]).String(), inTH, session)

		ssrcs[1] = th.SSRC

		res = doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

		var ri headers.RTPInfo
		err = ri.Unmarshal(res.Header["RTP-Info"])
		require.NoError(t, err)

		return &ri, ssrcs
	}

	forma := &format.Generic{
		PayloadTyp: 96,
		RTPMa:      "private/90000",
	}
	err := forma.Init()
	require.NoError(t, err)

	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
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

	stream = &ServerStream{
		Server: s,
		Desc: &description.Session{
			Medias: []*description.Media{
				{
					Type:    "application",
					Formats: []format.Format{forma},
				},
				{
					Type:    "application",
					Formats: []format.Format{forma},
				},
			},
		},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	err = stream.WritePacketRTP(stream.Description().Medias[0], &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 556,
			Timestamp:      984512368,
			SSRC:           96342362,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	rtpInfo, ssrcs := getInfos()
	require.True(t, strings.HasPrefix(mustParseURL((*rtpInfo)[0].URL).Path, "/teststream/trackID="))
	require.Equal(t, &headers.RTPInfo{
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   mustParseURL((*rtpInfo)[0].URL).Path,
			}).String(),
			SequenceNumber: uint16Ptr(557),
			Timestamp:      (*rtpInfo)[0].Timestamp,
		},
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   mustParseURL((*rtpInfo)[1].URL).Path,
			}).String(),
		},
	}, rtpInfo)
	require.Equal(t, []*uint32{
		uint32Ptr(96342362),
		nil,
	}, ssrcs)

	err = stream.WritePacketRTP(stream.Description().Medias[1], &rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			SequenceNumber: 87,
			Timestamp:      756436454,
			SSRC:           536474323,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	rtpInfo, ssrcs = getInfos()
	require.True(t, strings.HasPrefix(mustParseURL((*rtpInfo)[0].URL).Path, "/teststream/trackID="))
	require.Equal(t, &headers.RTPInfo{
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   mustParseURL((*rtpInfo)[0].URL).Path,
			}).String(),
			SequenceNumber: uint16Ptr(557),
			Timestamp:      (*rtpInfo)[0].Timestamp,
		},
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   mustParseURL((*rtpInfo)[1].URL).Path,
			}).String(),
			SequenceNumber: uint16Ptr(88),
			Timestamp:      (*rtpInfo)[1].Timestamp,
		},
	}, rtpInfo)
	require.Equal(t, []*uint32{
		uint32Ptr(96342362),
		uint32Ptr(536474323),
	}, ssrcs)
}

func TestServerPlayNoInterleavedIDs(t *testing.T) {
	forma := &format.Generic{
		PayloadTyp: 96,
		RTPMa:      "private/90000",
	}
	err := forma.Init()
	require.NoError(t, err)

	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
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

	stream = &ServerStream{
		Server: s,
		Desc: &description.Session{
			Medias: []*description.Media{
				{
					Type:    "application",
					Formats: []format.Format{forma},
				},
				{
					Type:    "application",
					Formats: []format.Format{forma},
				},
			},
		},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTH := &headers.Transport{
		Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:     transportModePtr(headers.TransportModePlay),
		Protocol: headers.TransportProtocolTCP,
	}

	res, th := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	require.Equal(t, &[2]int{0, 1}, th.InterleavedIDs)

	session := readSession(t, res)

	_, th = doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[1]).String(), inTH, session)

	require.Equal(t, &[2]int{2, 3}, th.InterleavedIDs)

	doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

	for i := 0; i < 2; i++ {
		err := stream.WritePacketRTP(stream.Description().Medias[i], &testRTPPacket)
		require.NoError(t, err)

		f, err := conn.ReadInterleavedFrame()
		require.NoError(t, err)
		require.Equal(t, i*2, f.Channel)
		require.Equal(t, testRTPPacketMarshaled, f.Payload)
	}
}

func TestServerPlayStreamStats(t *testing.T) {
	var stream *ServerStream

	s := &Server{
		RTSPAddress:       "localhost:8554",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8000,
		MulticastRTCPPort: 8001,
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	stream = &ServerStream{
		Server: s,
		Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	for _, transport := range []string{"tcp", "multicast"} {
		var nconn net.Conn
		nconn, err = net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		desc := doDescribe(t, conn)

		inTH := &headers.Transport{
			Mode: transportModePtr(headers.TransportModePlay),
		}

		if transport == "multicast" {
			v := headers.TransportDeliveryMulticast
			inTH.Delivery = &v
			inTH.Protocol = headers.TransportProtocolUDP
		} else {
			v := headers.TransportDeliveryUnicast
			inTH.Delivery = &v
			inTH.Protocol = headers.TransportProtocolTCP
			inTH.InterleavedIDs = &[2]int{0, 1}
		}

		res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

		session := readSession(t, res)

		doPlay(t, conn, "rtsp://localhost:8554/teststream", session)
	}

	err = stream.WritePacketRTP(stream.Description().Medias[0], &testRTPPacket)
	require.NoError(t, err)

	st := stream.Stats()
	require.Equal(t, uint64(16*2), st.BytesSent)
}
