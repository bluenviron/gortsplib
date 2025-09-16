package gortsplib

import (
	"bufio"
	"bytes"
	"crypto/rand"
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

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
	"github.com/bluenviron/gortsplib/v5/pkg/mikey"
	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
	"github.com/bluenviron/gortsplib/v5/pkg/sdp"
)

func multicastCapableIP(t *testing.T) string {
	intfs, err := net.Interfaces()
	require.NoError(t, err)

	for _, intf := range intfs {
		if (intf.Flags & net.FlagMulticast) != 0 {
			var addrs []net.Addr
			addrs, err = intf.Addrs()
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			desc := doDescribe(t, conn, false)

			th := &headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       ptrOf(headers.TransportDeliveryUnicast),
				Mode:           ptrOf(headers.TransportModePlay),
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
							require.EqualError(t, ctx.Error, "can't setup medias with different transports")
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			var desc *description.Session
			var th *headers.Transport
			var res *base.Response

			switch ca {
			case "invalid path":
				th = &headers.Transport{
					Protocol:    headers.TransportProtocolUDP,
					Delivery:    ptrOf(headers.TransportDeliveryUnicast),
					Mode:        ptrOf(headers.TransportModePlay),
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
				desc = doDescribe(t, conn, false)

				th = &headers.Transport{
					Protocol:    headers.TransportProtocolUDP,
					Delivery:    ptrOf(headers.TransportDeliveryUnicast),
					Mode:        ptrOf(headers.TransportModePlay),
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
		var nconn net.Conn
		nconn, err = net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		inTH := &headers.Transport{
			Delivery:    ptrOf(headers.TransportDeliveryUnicast),
			Mode:        ptrOf(headers.TransportModePlay),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		desc := doDescribe(t, conn, false)

		var res *base.Response
		res, err = writeReqReadRes(conn, base.Request{
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
	for _, ca := range []struct {
		scheme    string
		transport string
		secure    string
	}{
		{
			"rtsp",
			"udp",
			"unsecure",
		},
		{
			"rtsp",
			"multicast",
			"unsecure",
		},
		{
			"rtsp",
			"tcp",
			"unsecure",
		},
		{
			"rtsps",
			"tcp",
			"unsecure",
		},
		{
			"rtsps",
			"udp",
			"secure",
		},
		{
			"rtsps",
			"multicast",
			"secure",
		},
		{
			"rtsps",
			"tcp",
			"secure",
		},
	} {
		t.Run(ca.scheme+"_"+ca.transport+"_"+ca.secure, func(t *testing.T) {
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
						require.Greater(t, s.BytesSent, uint64(800))
						require.Less(t, s.BytesSent, uint64(1600))
						require.Greater(t, s.BytesReceived, uint64(400))
						require.Less(t, s.BytesReceived, uint64(950))

						close(nconnClosed)
					},
					onSessionOpen: func(_ *ServerHandlerOnSessionOpenCtx) {
						close(sessionOpened)
					},
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						if ca.transport != "multicast" {
							s := ctx.Session.Stats()
							require.Greater(t, s.BytesSent, uint64(50))
							require.Less(t, s.BytesSent, uint64(130))
							require.Greater(t, s.BytesReceived, uint64(15))
							require.Less(t, s.BytesReceived, uint64(35))
						}

						close(sessionClosed)
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						// test that properties can be accessed in parallel
						go func() {
							ctx.Conn.Session()
							ctx.Conn.Stats()
							ctx.Session.State()
							ctx.Session.Stats()
							ctx.Session.Medias()
							ctx.Session.Path()
							ctx.Session.Query()
							ctx.Session.Stream()
							ctx.Session.Transport()
						}()

						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						require.NotNil(t, ctx.Conn.Session())

						var proto Protocol
						switch ca.transport {
						case "udp":
							proto = ProtocolUDP

						case "tcp":
							proto = ProtocolTCP

						case "multicast":
							proto = ProtocolUDPMulticast
						}

						var profile headers.TransportProfile
						if ca.secure == "secure" {
							profile = headers.TransportProfileSAVP
						} else {
							profile = headers.TransportProfileAVP
						}

						require.Equal(t, &SessionTransport{
							Protocol: proto,
							Profile:  profile,
						}, ctx.Session.Transport())

						require.Equal(t, "param=value", ctx.Session.Query())
						require.Equal(t, stream.Desc.Medias, ctx.Session.Medias())

						// send RTCP packets directly to the session.
						// these are sent after the response, only if onPlay returns StatusOK.
						if ca.transport != "multicast" {
							err := ctx.Session.WritePacketRTCP(stream.Desc.Medias[0], &testRTCPPacket)
							require.NoError(t, err)
						}

						ctx.Session.OnPacketRTCPAny(func(medi *description.Media, pkt rtcp.Packet) {
							// ignore multicast loopback
							if ca.secure == "unsecure" && ca.transport == "multicast" && atomic.AddUint64(&counter, 1) > 1 {
								return
							}

							require.Equal(t, stream.Desc.Medias[0], medi)
							require.Equal(t, &testRTCPPacket, pkt)
							close(framesReceived)
						})

						// the session is added to the stream only after onPlay returns
						// with StatusOK; therefore we must wait before calling
						// ServerStream.WritePacket*()
						go func() {
							time.Sleep(500 * time.Millisecond)
							err := stream.WritePacketRTCP(stream.Desc.Medias[0], &testRTCPPacket)
							require.NoError(t, err)
							err = stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
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

			switch ca.transport {
			case "udp":
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"

			case "multicast":
				s.MulticastIPRange = "224.1.0.0/16"
				s.MulticastRTPPort = 8000
				s.MulticastRTCPPort = 8001
			}

			if ca.scheme == "rtsps" {
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
				if ca.scheme == "rtsps" {
					return tls.Client(nconn, &tls.Config{InsecureSkipVerify: true})
				}
				return nconn
			}()
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			<-nconnOpened

			desc := doDescribe(t, conn, false)

			if ca.secure == "secure" {
				require.Equal(t, headers.TransportProfileSAVP, desc.Medias[0].Profile)
				require.NotEmpty(t, desc.Medias[0].KeyMgmtMikey)
			}

			inTH := &headers.Transport{
				Mode: ptrOf(headers.TransportModePlay),
			}

			switch ca.transport {
			case "udp":
				v := headers.TransportDeliveryUnicast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}

			case "multicast":
				v := headers.TransportDeliveryMulticast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP

			case "tcp":
				v := headers.TransportDeliveryUnicast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{5, 6} // odd value
			}

			h := base.Header{
				"CSeq": base.HeaderValue{"1"},
			}

			var srtpOutCtx *wrappedSRTPContext

			if ca.secure == "secure" {
				inTH.Profile = headers.TransportProfileSAVP

				key := make([]byte, srtpKeyLength)
				_, err = rand.Read(key)
				require.NoError(t, err)

				srtpOutCtx = &wrappedSRTPContext{
					key:   key,
					ssrcs: []uint32{2345423},
				}
				err = srtpOutCtx.initialize()
				require.NoError(t, err)

				var mikeyMsg *mikey.Message
				mikeyMsg, err = mikeyGenerate(srtpOutCtx)
				require.NoError(t, err)

				var enc base.HeaderValue
				enc, err = headers.KeyMgmt{
					URL:          mediaURL(t, desc.BaseURL, desc.Medias[0]).String(),
					MikeyMessage: mikeyMsg,
				}.Marshal()
				require.NoError(t, err)
				h["KeyMgmt"] = enc
			}

			h["Transport"] = inTH.Marshal()

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
				Header: h,
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var th headers.Transport
			err = th.Unmarshal(res.Header["Transport"])
			require.NoError(t, err)

			var srtpInCtx *wrappedSRTPContext

			if ca.secure == "secure" {
				require.Equal(t, headers.TransportProfileSAVP, th.Profile)

				var keyMgmt headers.KeyMgmt
				err = keyMgmt.Unmarshal(res.Header["KeyMgmt"])
				require.NoError(t, err)

				pl1, _ := mikeyGetPayload[*mikey.PayloadKEMAC](keyMgmt.MikeyMessage)
				pl2, _ := mikeyGetPayload[*mikey.PayloadKEMAC](desc.Medias[0].KeyMgmtMikey)
				require.Equal(t, pl1, pl2)

				srtpInCtx, err = mikeyToContext(keyMgmt.MikeyMessage)
				require.NoError(t, err)
			}

			var l1 net.PacketConn
			var l2 net.PacketConn

			switch ca.transport {
			case "udp":
				require.Equal(t, headers.TransportProtocolUDP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)

				l1, err = net.ListenPacket("udp", listenIP+":35466")
				require.NoError(t, err)
				defer l1.Close()

				l2, err = net.ListenPacket("udp", listenIP+":35467")
				require.NoError(t, err)
				defer l2.Close()

			case "multicast": //nolint:dupl
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
					err = p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP(*th.Destination2)})
					require.NoError(t, err)
				}

				l2, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[1]), 10))
				require.NoError(t, err)
				defer l2.Close()

				p = ipv4.NewPacketConn(l2)

				intfs, err = net.Interfaces()
				require.NoError(t, err)

				for _, intf := range intfs {
					err = p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP(*th.Destination2)})
					require.NoError(t, err)
				}

			case "tcp":
				require.Equal(t, headers.TransportProtocolTCP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)
			}

			<-sessionOpened

			session := readSession(t, res)

			doPlay(t, conn, "rtsp://"+listenIP+":8554/teststream", session)

			// server -> client (direct)

			if ca.transport != "multicast" {
				var buf []byte

				switch ca.transport {
				case "udp":
					buf = make([]byte, 2048)
					var n int
					n, _, err = l2.ReadFrom(buf)
					require.NoError(t, err)
					buf = buf[:n]

				case "tcp":
					var f *base.InterleavedFrame
					f, err = conn.ReadInterleavedFrame()
					require.NoError(t, err)
					require.Equal(t, 6, f.Channel)
					buf = f.Payload
				}

				if ca.secure == "secure" {
					buf, err = srtpInCtx.decryptRTCP(buf, buf, nil)
					require.NoError(t, err)
				}

				require.Equal(t, testRTCPPacketMarshaled, buf)
			}

			// server -> client (through stream)

			var buf1 []byte
			var buf2 []byte

			switch ca.transport {
			case "udp", "multicast":
				buf1 = make([]byte, 2048)
				var n int
				n, _, err = l1.ReadFrom(buf1)
				require.NoError(t, err)
				buf1 = buf1[:n]

				buf2 = make([]byte, 2048)
				n, _, err = l2.ReadFrom(buf2)
				require.NoError(t, err)
				buf2 = buf2[:n]

			case "tcp":
				var f *base.InterleavedFrame
				f, err = conn.ReadInterleavedFrame()
				require.NoError(t, err)
				require.Equal(t, 6, f.Channel)
				buf2 = f.Payload

				f, err = conn.ReadInterleavedFrame()
				require.NoError(t, err)
				require.Equal(t, 5, f.Channel)
				buf1 = f.Payload
			}

			if ca.secure == "secure" {
				buf1, err = srtpInCtx.decryptRTP(buf1, buf1, nil)
				require.NoError(t, err)
			}

			var pkt rtp.Packet
			err = pkt.Unmarshal(buf1)
			require.NoError(t, err)
			pkt.SSRC = testRTPPacket.SSRC
			require.Equal(t, testRTPPacket, pkt)

			if ca.secure == "secure" {
				buf2, err = srtpInCtx.decryptRTCP(buf2, buf2, nil)
				require.NoError(t, err)
			}

			require.Equal(t, testRTCPPacketMarshaled, buf2)

			// client -> server

			buf := testRTCPPacketMarshaled

			if ca.secure == "secure" {
				encr := make([]byte, 2000)
				encr, err = srtpOutCtx.encryptRTCP(encr, buf, nil)
				require.NoError(t, err)
				buf = encr
			}

			switch ca.transport {
			case "udp":
				_, err = l2.WriteTo(buf, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
				require.NoError(t, err)

			case "multicast":
				_, err = l2.WriteTo(buf, &net.UDPAddr{
					IP:   net.ParseIP(*th.Destination2),
					Port: th.Ports[1],
				})
				require.NoError(t, err)

			case "tcp":
				err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 6,
					Payload: buf,
				}, make([]byte, 1024))
				require.NoError(t, err)
			}

			<-framesReceived

			// ping

			switch ca.transport {
			case "udp", "multicast":
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
								err := stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
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
				var nconn net.Conn
				nconn, err = net.Dial("tcp", listenIP+":8554")
				require.NoError(t, err)
				defer nconn.Close()

				conn := conn.NewConn(bufio.NewReader(nconn), nconn)

				desc := doDescribe(t, conn, false)

				inTH := &headers.Transport{
					Mode: ptrOf(headers.TransportModePlay),
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

				case "multicast": //nolint:dupl
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
						err = p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP(*th.Destination2)})
						require.NoError(t, err)
					}

					l2, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[1]), 10))
					require.NoError(t, err)
					defer l2.Close()

					p = ipv4.NewPacketConn(l2)

					intfs, err = net.Interfaces()
					require.NoError(t, err)

					for _, intf := range intfs {
						err = p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP(*th.Destination2)})
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			desc := doDescribe(t, conn, false)

			inTH := &headers.Transport{
				Mode:     ptrOf(headers.TransportModePlay),
				Delivery: ptrOf(headers.TransportDeliveryUnicast),
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
				_, err = l2.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[1],
				})
				require.NoError(t, err)

			case ca.proto == "udp" && ca.name == "rtcp too big":
				_, err = l2.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[1],
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			desc := doDescribe(t, conn, false)

			inTH := &headers.Transport{
				Mode:     ptrOf(headers.TransportModePlay),
				Delivery: ptrOf(headers.TransportDeliveryUnicast),
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
				stream.Desc.Medias[0],
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
			require.Equal(t, []rtcp.Packet{
				&rtcp.SenderReport{
					SSRC:        packets[0].(*rtcp.SenderReport).SSRC,
					NTPTime:     ntp.Encode(time.Date(2017, 8, 10, 12, 22, 30, 0, time.UTC)),
					RTPTime:     240000 + 90000*30,
					PacketCount: 1,
					OctetCount:  1,
				},
			}, packets)

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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)
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

					err := stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
					require.NoError(t, err)

					ti := time.NewTicker(50 * time.Millisecond)
					defer ti.Stop()

					for {
						select {
						case <-ti.C:
							err = stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	desc := doDescribe(t, conn, false)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       ptrOf(headers.TransportDeliveryUnicast),
		Mode:           ptrOf(headers.TransportModePlay),
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
								err := stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
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
			onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
				// test that properties can be accessed in parallel
				go func() {
					ctx.Session.State()
					ctx.Session.Stats()
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
		Desc:   &description.Session{Medias: []*description.Media{testH264Media}},
	}
	err = stream.Initialize()
	require.NoError(t, err)
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	desc := doDescribe(t, conn, false)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       ptrOf(headers.TransportDeliveryUnicast),
		Mode:           ptrOf(headers.TransportModePlay),
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
											err := stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
											require.NoError(t, err)
										} else {
											err := ctx.Session.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			desc := doDescribe(t, conn, false)

			inTH := &headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       ptrOf(headers.TransportDeliveryUnicast),
				Mode:           ptrOf(headers.TransportModePlay),
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			desc := doDescribe(t, conn, false)

			inTH := &headers.Transport{
				Mode: ptrOf(headers.TransportModePlay),
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			desc := doDescribe(t, conn, false)

			inTH := &headers.Transport{
				Delivery: ptrOf(headers.TransportDeliveryUnicast),
				Mode:     ptrOf(headers.TransportModePlay),
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
		var nconn net.Conn
		nconn, err = net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		desc := doDescribe(t, conn, false)

		inTH := &headers.Transport{
			Delivery:    ptrOf(headers.TransportDeliveryUnicast),
			Mode:        ptrOf(headers.TransportModePlay),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

		session := readSession(t, res)

		doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

		sxID = session
	}()

	func() {
		var nconn net.Conn
		nconn, err = net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		var res *base.Response
		res, err = writeReqReadRes(conn, base.Request{
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
					err := stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
					require.NoError(t, err)
					err = stream.WritePacketRTP(stream.Desc.Medias[1], &testRTPPacket)
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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	desc := doDescribe(t, conn, false)

	inTH := &headers.Transport{
		Delivery:       ptrOf(headers.TransportDeliveryUnicast),
		Mode:           ptrOf(headers.TransportModePlay),
		Protocol:       headers.TransportProtocolTCP,
		InterleavedIDs: &[2]int{4, 5},
	}

	res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	session := readSession(t, res)

	doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

	f, err := conn.ReadInterleavedFrame()
	require.NoError(t, err)
	require.Equal(t, 4, f.Channel)

	var pkt rtp.Packet
	err = pkt.Unmarshal(f.Payload)
	require.NoError(t, err)
	pkt.SSRC = testRTPPacket.SSRC
	require.Equal(t, testRTPPacket, pkt)
}

func TestServerPlayAdditionalInfos(t *testing.T) {
	getInfos := func() (*headers.RTPInfo, []*uint32) {
		nconn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		desc := doDescribe(t, conn, false)

		inTH := &headers.Transport{
			Delivery:       ptrOf(headers.TransportDeliveryUnicast),
			Mode:           ptrOf(headers.TransportModePlay),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		}

		res, th := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

		ssrcs := make([]*uint32, 2)
		ssrcs[0] = th.SSRC

		inTH = &headers.Transport{
			Delivery:       ptrOf(headers.TransportDeliveryUnicast),
			Mode:           ptrOf(headers.TransportModePlay),
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

	err = stream.WritePacketRTP(stream.Desc.Medias[0], &rtp.Packet{
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
			SequenceNumber: ptrOf(uint16(557)),
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
	require.Len(t, ssrcs, 2)
	require.NotNil(t, ssrcs[0])
	require.NotNil(t, ssrcs[1])

	err = stream.WritePacketRTP(stream.Desc.Medias[1], &rtp.Packet{
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
			SequenceNumber: ptrOf(uint16(557)),
			Timestamp:      (*rtpInfo)[0].Timestamp,
		},
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   mustParseURL((*rtpInfo)[1].URL).Path,
			}).String(),
			SequenceNumber: ptrOf(uint16(88)),
			Timestamp:      (*rtpInfo)[1].Timestamp,
		},
	}, rtpInfo)
	require.Len(t, ssrcs, 2)
	require.NotNil(t, ssrcs[0])
	require.NotNil(t, ssrcs[1])
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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	desc := doDescribe(t, conn, false)

	inTH := &headers.Transport{
		Delivery: ptrOf(headers.TransportDeliveryUnicast),
		Mode:     ptrOf(headers.TransportModePlay),
		Protocol: headers.TransportProtocolTCP,
	}

	res, th := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	require.Equal(t, &[2]int{0, 1}, th.InterleavedIDs)

	session := readSession(t, res)

	_, th = doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[1]).String(), inTH, session)

	require.Equal(t, &[2]int{2, 3}, th.InterleavedIDs)

	doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

	for i := range 2 {
		err = stream.WritePacketRTP(stream.Desc.Medias[i], &testRTPPacket)
		require.NoError(t, err)

		var f *base.InterleavedFrame
		f, err = conn.ReadInterleavedFrame()
		require.NoError(t, err)
		require.Equal(t, i*2, f.Channel)

		var pkt rtp.Packet
		err = pkt.Unmarshal(f.Payload)
		require.NoError(t, err)
		pkt.SSRC = testRTPPacket.SSRC
		require.Equal(t, testRTPPacket, pkt)
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
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		desc := doDescribe(t, conn, false)

		inTH := &headers.Transport{
			Mode: ptrOf(headers.TransportModePlay),
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

	err = stream.WritePacketRTP(stream.Desc.Medias[0], &testRTPPacket)
	require.NoError(t, err)

	st := stream.Stats()
	require.Equal(t, &ServerStreamStats{
		BytesSent:      32,
		RTPPacketsSent: 2,
		Medias: map[*description.Media]ServerStreamStatsMedia{
			stream.Desc.Medias[0]: {
				BytesSent:       32,
				RTCPPacketsSent: 0,
				Formats: map[format.Format]ServerStreamStatsFormat{
					stream.Desc.Medias[0].Formats[0]: {
						RTPPacketsSent: 2,
						LocalSSRC: st.Medias[stream.Desc.Medias[0]].
							Formats[stream.Desc.Medias[0].Formats[0]].LocalSSRC,
					},
				},
			},
		},
	}, st)
}

func TestServerPlayBackChannel(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			serverOk := make(chan struct{})
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
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						ctx.Session.OnPacketRTPAny(func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
							close(serverOk)
						})

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress: "127.0.0.1:8554",
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
				Desc: &description.Session{Medias: []*description.Media{
					testH264Media,
					{
						Type:          description.MediaTypeAudio,
						IsBackChannel: true,
						Formats: []format.Format{&format.G711{
							PayloadTyp:   8,
							MULaw:        false,
							SampleRate:   8000,
							ChannelCount: 1,
						}},
					},
				}},
			}
			err = stream.Initialize()
			require.NoError(t, err)
			defer stream.Close()

			nconn, err := net.Dial("tcp", "127.0.0.1:8554")
			require.NoError(t, err)
			defer nconn.Close()

			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			desc := doDescribe(t, conn, true)

			var session string
			var serverPorts [2]*[2]int
			var l1s [2]net.PacketConn
			var l2s [2]net.PacketConn

			for i := 0; i < 2; i++ {
				inTH := &headers.Transport{
					Mode: ptrOf(headers.TransportModePlay),
				}

				if transport == "udp" {
					v := headers.TransportDeliveryUnicast
					inTH.Delivery = &v
					inTH.Protocol = headers.TransportProtocolUDP
					inTH.ClientPorts = &[2]int{35466 + i*2, 35467 + i*2}
				} else {
					v := headers.TransportDeliveryUnicast
					inTH.Delivery = &v
					inTH.Protocol = headers.TransportProtocolTCP
					inTH.InterleavedIDs = &[2]int{0 + i*2, 1 + i*2}
				}

				res, th := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[i]).String(), inTH, "")

				if transport == "udp" {
					serverPorts[i] = th.ServerPorts

					l1s[i], err = net.ListenPacket("udp", net.JoinHostPort("127.0.0.1", strconv.FormatInt(int64(35466+i*2), 10)))
					require.NoError(t, err)
					defer l1s[i].Close()

					l2s[i], err = net.ListenPacket("udp", net.JoinHostPort("127.0.0.1", strconv.FormatInt(int64(35467+i*2), 10)))
					require.NoError(t, err)
					defer l2s[i].Close()
				}

				session = readSession(t, res)
			}

			doPlay(t, conn, "rtsp://127.0.0.1:8554/teststream", session)

			// client -> server

			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:     2,
					PayloadType: 8,
				},
				Payload: []byte{1, 2, 3, 4},
			}
			buf, err := pkt.Marshal()
			require.NoError(t, err)

			if transport == "udp" {
				_, err = l1s[1].WriteTo(buf, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: serverPorts[1][0],
				})
				require.NoError(t, err)
			} else {
				err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 2,
					Payload: buf,
				}, make([]byte, 1024))
				require.NoError(t, err)
			}

			<-serverOk

			doTeardown(t, conn, "rtsp://127.0.0.1:8554/teststream", session)
		})
	}
}
