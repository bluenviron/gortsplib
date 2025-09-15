package gortsplib

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
	"github.com/bluenviron/gortsplib/v5/pkg/mikey"
	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
	"github.com/bluenviron/gortsplib/v5/pkg/sdp"
)

func doAnnounce(t *testing.T, conn *conn.Conn, u string, medias []*description.Media) {
	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL(u),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: mediasToSDP(medias),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func doRecord(t *testing.T, conn *conn.Conn, u string, session string) {
	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Record,
		URL:    mustParseURL(u),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"1"},
			"Session": base.HeaderValue{session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerRecordErrorAnnounce(t *testing.T) {
	for _, ca := range []struct {
		name string
		req  base.Request
		err  string
	}{
		{
			"missing content-type",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq": base.HeaderValue{"1"},
				},
			},
			"Content-Type header is missing",
		},
		{
			"invalid content-type",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"aa"},
				},
			},
			"unsupported Content-Type header '[aa]'",
		},
		{
			"invalid sdp",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: []byte{0x01, 0x02, 0x03, 0x04},
			},
			"invalid SDP: invalid line: (\x01\x02\x03\x04)",
		},
		{
			"invalid session",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: []byte("v=0\r\n" +
					"o=- 0 0 IN IP4 127.0.0.1\r\n" +
					"s=-\r\n" +
					"c=IN IP4 0.0.0.0\r\n" +
					"t=0 0\r\n" +
					"m=video 0 RTP/AVP 96\r\n" +
					"a=control\r\n" +
					"a=rtpmap:97 H264/90000\r\n" +
					"a=fmtp:aa packetization-mode=1; profile-level-id=4D002A; " +
					"sprop-parameter-sets=Z00AKp2oHgCJ+WbgICAgQA==,aO48gA==\r\n",
				),
			},
			"invalid SDP: media 1 is invalid: clock rate not found",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			nconnClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						require.EqualError(t, ctx.Error, ca.err)
						close(nconnClosed)
					},
					onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			_, err = writeReqReadRes(conn, ca.req)
			require.NoError(t, err)

			<-nconnClosed
		})
	}
}

func TestServerRecordErrorSetup(t *testing.T) {
	for _, ca := range []struct {
		name string
		err  string
	}{
		{
			"invalid transport",
			"transport header contains a invalid mode (null)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						require.EqualError(t, ctx.Error, ca.err)
					},
					onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
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
				RTSPAddress:    "localhost:8554",
				UDPRTPAddress:  "127.0.0.1:8000",
				UDPRTCPAddress: "127.0.0.1:8001",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			medias := []*description.Media{testH264Media}

			doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

			var inTH *headers.Transport

			if ca.name == "invalid transport" {
				inTH = &headers.Transport{
					Delivery:    ptrOf(headers.TransportDeliveryUnicast),
					Mode:        nil,
					Protocol:    headers.TransportProtocolUDP,
					ClientPorts: &[2]int{35466, 35467},
				}
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/" + medias[0].Control),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.NotEqual(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerRecordPath(t *testing.T) {
	for _, ca := range []struct {
		name        string
		control     string
		announceURL string
		setupURL    string
		path        string
		query       string
	}{
		{
			"normal",
			"bbb=ccc",
			"rtsp://localhost:8554/teststream",
			"rtsp://localhost:8554/teststream/bbb=ccc",
			"/teststream",
			"",
		},
		{
			"subpath",
			"ddd=eee",
			"rtsp://localhost:8554/test/stream",
			"rtsp://localhost:8554/test/stream/ddd=eee",
			"/test/stream",
			"",
		},
		{
			"subpath and query, ffmpeg format",
			"fff=ggg",
			"rtsp://localhost:8554/test/stream?testing=0",
			"rtsp://localhost:8554/test/stream?testing=0/fff=ggg",
			"/test/stream",
			"testing=0",
		},
		{
			"subpath and query, gstreamer format",
			"fff=ggg",
			"rtsp://localhost:8554/test/stream?testing=0",
			"rtsp://localhost:8554/test/stream/fff=ggg?testing=0",
			"/test/stream",
			"testing=0",
		},
		{
			"no path",
			"streamid=1",
			"rtsp://localhost:8554",
			"rtsp://localhost:8554/streamid=1",
			"",
			"",
		},
		{
			"single slash",
			"streamid=1",
			"rtsp://localhost:8554/",
			"rtsp://localhost:8554//streamid=1",
			"/",
			"",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var s *Server
			s = &Server{
				Handler: &testServerHandler{
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						// make sure that media URLs are not overridden by ServerStream.Initialize()
						stream := &ServerStream{
							Server: s,
							Desc:   ctx.Description,
						}
						err := stream.Initialize()
						require.NoError(t, err)
						defer stream.Close()

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						require.Equal(t, ca.path, ctx.Path)
						require.Equal(t, ca.query, ctx.Query)

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						require.Equal(t, ca.path, ctx.Path)
						require.Equal(t, ca.query, ctx.Query)

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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			media := testH264Media
			media.Control = ca.control

			enc, err := media.Marshal2()
			require.NoError(t, err)

			sout := &sdp.SessionDescription{
				SessionName: psdp.SessionName("Stream"),
				Origin: psdp.Origin{
					Username:       "-",
					NetworkType:    "IN",
					AddressType:    "IP4",
					UnicastAddress: "127.0.0.1",
				},
				TimeDescriptions: []psdp.TimeDescription{
					{Timing: psdp.Timing{}},
				},
				MediaDescriptions: []*psdp.MediaDescription{enc},
			}

			byts, _ := sout.Marshal()

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL(ca.announceURL),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: byts,
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			th := &headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       ptrOf(headers.TransportDeliveryUnicast),
				Mode:           ptrOf(headers.TransportModeRecord),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, _ = doSetup(t, conn, ca.setupURL, th, "")

			session := readSession(t, res)

			doRecord(t, conn, ca.announceURL, session)
		})
	}
}

func TestServerRecordErrorSetupMediaTwice(t *testing.T) {
	serverErr := make(chan error)

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				serverErr <- ctx.Error
			},
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	medias := []*description.Media{testH264Media}

	doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       ptrOf(headers.TransportDeliveryUnicast),
		Mode:           ptrOf(headers.TransportModeRecord),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, _ := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

	session := readSession(t, res)

	inTH = &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       ptrOf(headers.TransportDeliveryUnicast),
		Mode:           ptrOf(headers.TransportModeRecord),
		InterleavedIDs: &[2]int{2, 3},
	}

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/" + medias[0].Control),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"3"},
			"Transport": inTH.Marshal(),
			"Session":   base.HeaderValue{session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.EqualError(t, err, "media has already been setup")
}

func TestServerRecordErrorRecordPartialMedias(t *testing.T) {
	serverErr := make(chan error)

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				serverErr <- ctx.Error
			},
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	forma := &format.Generic{
		PayloadTyp: 96,
		RTPMa:      "private/90000",
	}
	err = forma.Init()
	require.NoError(t, err)

	medias := []*description.Media{
		{
			Type:    "application",
			Formats: []format.Format{forma},
		},
		{
			Type:    "application",
			Formats: []format.Format{forma},
		},
	}

	doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       ptrOf(headers.TransportDeliveryUnicast),
		Mode:           ptrOf(headers.TransportModeRecord),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, _ := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

	session := readSession(t, res)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Record,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": base.HeaderValue{session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.EqualError(t, err, "not all announced medias have been setup")
}

func TestServerRecord(t *testing.T) {
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
			"tcp",
			"secure",
		},
	} {
		t.Run(ca.scheme+"_"+ca.transport+"_"+ca.secure, func(t *testing.T) {
			nconnOpened := make(chan struct{})
			nconnClosed := make(chan struct{})
			sessionOpened := make(chan struct{})
			sessionClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnOpen: func(_ *ServerHandlerOnConnOpenCtx) {
						close(nconnOpened)
					},
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						s := ctx.Conn.Stats()
						require.Greater(t, s.BytesSent, uint64(510))
						require.Less(t, s.BytesSent, uint64(1100))
						require.Greater(t, s.BytesReceived, uint64(1000))
						require.Less(t, s.BytesReceived, uint64(1800))

						close(nconnClosed)
					},
					onSessionOpen: func(_ *ServerHandlerOnSessionOpenCtx) {
						close(sessionOpened)
					},
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						s := ctx.Session.Stats()
						require.Greater(t, s.BytesSent, uint64(75))
						require.Less(t, s.BytesSent, uint64(140))
						require.Greater(t, s.BytesReceived, uint64(70))
						require.Less(t, s.BytesReceived, uint64(130))

						close(sessionClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						// test that properties can be accessed in parallel
						go func() {
							ctx.Session.State()
							ctx.Session.Stats()
							ctx.Session.AnnouncedDescription()
						}()

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						var proto TransportProtocol
						switch ca.transport {
						case "udp":
							proto = TransportUDP

						case "tcp":
							proto = TransportTCP
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
						require.Equal(t, ctx.Session.AnnouncedDescription().Medias, ctx.Session.Medias())

						// queue sending of RTCP packets.
						// these are sent after the response, only if onRecord returns StatusOK.
						err := ctx.Session.WritePacketRTCP(ctx.Session.AnnouncedDescription().Medias[0], &testRTCPPacket)
						require.NoError(t, err)
						err = ctx.Session.WritePacketRTCP(ctx.Session.AnnouncedDescription().Medias[1], &testRTCPPacket)
						require.NoError(t, err)

						for i := 0; i < 2; i++ {
							ctx.Session.OnPacketRTP(
								ctx.Session.AnnouncedDescription().Medias[i],
								ctx.Session.AnnouncedDescription().Medias[i].Formats[0],
								func(pkt *rtp.Packet) {
									pkt.SSRC = testRTPPacket.SSRC
									require.Equal(t, &testRTPPacket, pkt)
								})

							ci := i
							ctx.Session.OnPacketRTCP(
								ctx.Session.AnnouncedDescription().Medias[i],
								func(pkt rtcp.Packet) {
									require.Equal(t, &testRTCPPacket, pkt)
									err2 := ctx.Session.WritePacketRTCP(ctx.Session.AnnouncedDescription().Medias[ci], &testRTCPPacket)
									require.NoError(t, err2)
								})
						}

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress: "localhost:8554",
			}

			if ca.transport == "udp" {
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"
			}

			if ca.scheme == "rtsps" {
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()

			nconn = func() net.Conn {
				if ca.scheme == "rtsps" {
					return tls.Client(nconn, &tls.Config{InsecureSkipVerify: true})
				}
				return nconn
			}()
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			<-nconnOpened

			medias := []*description.Media{
				{
					Type: description.MediaTypeVideo,
					Formats: []format.Format{&format.H264{
						PayloadTyp:        96,
						SPS:               testH264Media.Formats[0].(*format.H264).SPS,
						PPS:               testH264Media.Formats[0].(*format.H264).PPS,
						PacketizationMode: 1,
					}},
				},
				{
					Type: description.MediaTypeVideo,
					Formats: []format.Format{&format.H264{
						PayloadTyp:        96,
						SPS:               testH264Media.Formats[0].(*format.H264).SPS,
						PPS:               testH264Media.Formats[0].(*format.H264).PPS,
						PacketizationMode: 1,
					}},
				},
			}

			doAnnounce(t, conn, "rtsp://localhost:8554/teststream?param=value", medias)

			<-sessionOpened

			var l1s [2]net.PacketConn
			var l2s [2]net.PacketConn
			var session string
			var serverPorts [2]*[2]int
			var srtpOutCtx [2]*wrappedSRTPContext
			var srtpInCtx [2]*wrappedSRTPContext

			for i := 0; i < 2; i++ {
				inTH := &headers.Transport{
					Delivery: ptrOf(headers.TransportDeliveryUnicast),
					Mode:     ptrOf(headers.TransportModeRecord),
				}

				if ca.transport == "udp" {
					inTH.Protocol = headers.TransportProtocolUDP
					inTH.ClientPorts = &[2]int{35466 + i*2, 35467 + i*2}

					l1s[i], err = net.ListenPacket("udp", "localhost:"+strconv.FormatInt(int64(inTH.ClientPorts[0]), 10))
					require.NoError(t, err)
					defer l1s[i].Close()

					l2s[i], err = net.ListenPacket("udp", "localhost:"+strconv.FormatInt(int64(inTH.ClientPorts[1]), 10))
					require.NoError(t, err)
					defer l2s[i].Close()
				} else {
					inTH.Protocol = headers.TransportProtocolTCP
					inTH.InterleavedIDs = &[2]int{2 + i*2, 3 + i*2}
				}

				h := base.Header{
					"CSeq": base.HeaderValue{"1"},
				}

				if session != "" {
					h["Session"] = base.HeaderValue{session}
				}

				if ca.secure == "secure" {
					inTH.Profile = headers.TransportProfileSAVP

					key := make([]byte, srtpKeyLength)
					_, err = rand.Read(key)
					require.NoError(t, err)

					srtpOutCtx[i] = &wrappedSRTPContext{
						key:   key,
						ssrcs: []uint32{2345423},
					}
					err = srtpOutCtx[i].initialize()
					require.NoError(t, err)

					var mikeyMsg *mikey.Message
					mikeyMsg, err = mikeyGenerate(srtpOutCtx[i])
					require.NoError(t, err)

					var enc base.HeaderValue
					enc, err = headers.KeyMgmt{
						URL:          "rtsp://localhost:8554/teststream?param=value/" + medias[i].Control,
						MikeyMessage: mikeyMsg,
					}.Marshal()
					require.NoError(t, err)
					h["KeyMgmt"] = enc
				}

				h["Transport"] = inTH.Marshal()

				var res *base.Response
				res, err = writeReqReadRes(conn, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/teststream?param=value/" + medias[i].Control),
					Header: h,
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				var th headers.Transport
				err = th.Unmarshal(res.Header["Transport"])
				require.NoError(t, err)

				session = readSession(t, res)

				if ca.transport == "udp" {
					serverPorts[i] = th.ServerPorts
				}

				if ca.secure == "secure" {
					require.Equal(t, headers.TransportProfileSAVP, th.Profile)

					var keyMgmt headers.KeyMgmt
					err = keyMgmt.Unmarshal(res.Header["KeyMgmt"])
					require.NoError(t, err)

					srtpInCtx[i], err = mikeyToContext(keyMgmt.MikeyMessage)
					require.NoError(t, err)
				}
			}

			doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

			// skip firewall opening

			if ca.transport == "udp" {
				for i := 0; i < 2; i++ {
					buf := make([]byte, 2048)
					_, _, err = l2s[i].ReadFrom(buf)
					require.NoError(t, err)
				}
			}

			// server -> client

			for i := 0; i < 2; i++ {
				var buf []byte

				if ca.transport == "udp" {
					buf = make([]byte, 2048)
					var n int
					n, _, err = l2s[i].ReadFrom(buf)
					require.NoError(t, err)
					buf = buf[:n]
				} else {
					var f *base.InterleavedFrame
					f, err = conn.ReadInterleavedFrame()
					require.NoError(t, err)
					require.Equal(t, 3+i*2, f.Channel)
					buf = f.Payload
				}

				if ca.secure == "secure" {
					buf, err = srtpInCtx[i].decryptRTCP(buf, buf, nil)
					require.NoError(t, err)
				}

				require.Equal(t, testRTCPPacketMarshaled, buf)
			}

			// client -> server

			for i := 0; i < 2; i++ {
				buf1 := testRTPPacketMarshaled

				if ca.secure == "secure" {
					encr := make([]byte, 2000)
					encr, err = srtpOutCtx[i].encryptRTP(encr, buf1, nil)
					require.NoError(t, err)
					buf1 = encr
				}

				buf2 := testRTCPPacketMarshaled

				if ca.secure == "secure" {
					encr := make([]byte, 2000)
					encr, err = srtpOutCtx[i].encryptRTCP(encr, buf2, nil)
					require.NoError(t, err)
					buf2 = encr
				}

				if ca.transport == "udp" {
					_, err = l1s[i].WriteTo(buf1, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: serverPorts[i][0],
					})
					require.NoError(t, err)

					_, err = l2s[i].WriteTo(buf2, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: serverPorts[i][1],
					})
					require.NoError(t, err)
				} else {
					err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
						Channel: 2 + i*2,
						Payload: buf1,
					}, make([]byte, 1024))
					require.NoError(t, err)

					err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
						Channel: 3 + i*2,
						Payload: buf2,
					}, make([]byte, 1024))
					require.NoError(t, err)
				}
			}

			// server -> client

			for i := 0; i < 2; i++ {
				var buf []byte

				if ca.transport == "udp" {
					buf = make([]byte, 2048)
					var n int
					n, _, err = l2s[i].ReadFrom(buf)
					require.NoError(t, err)
					buf = buf[:n]
				} else {
					var f *base.InterleavedFrame
					f, err = conn.ReadInterleavedFrame()
					require.NoError(t, err)
					require.Equal(t, 3+i*2, f.Channel)
					buf = f.Payload
				}

				if ca.secure == "secure" {
					buf, err = srtpInCtx[i].decryptRTCP(buf, buf, nil)
					require.NoError(t, err)
				}

				require.Equal(t, testRTCPPacketMarshaled, buf)
			}

			doTeardown(t, conn, "rtsp://localhost:8554/teststream", session)

			<-sessionClosed

			nconn.Close()
			<-nconnClosed
		})
	}
}

func TestServerRecordErrorInvalidProtocol(t *testing.T) {
	errorRecv := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				require.EqualError(t, ctx.Error, "received unexpected interleaved frame")
				close(errorRecv)
			},
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	medias := []*description.Media{testH264Media}

	doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

	inTH := &headers.Transport{
		Delivery:    ptrOf(headers.TransportDeliveryUnicast),
		Mode:        ptrOf(headers.TransportModeRecord),
		Protocol:    headers.TransportProtocolUDP,
		ClientPorts: &[2]int{35466, 35467},
	}

	res, _ := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

	session := readSession(t, res)

	doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

	err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
		Channel: 0,
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}, make([]byte, 1024))
	require.NoError(t, err)

	<-errorRecv
}

func TestServerRecordRTCPReport(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		UDPRTPAddress:        "127.0.0.1:8000",
		UDPRTCPAddress:       "127.0.0.1:8001",
		RTSPAddress:          "localhost:8554",
		receiverReportPeriod: 500 * time.Millisecond,
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	medias := []*description.Media{testH264Media}

	doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

	l1, err := net.ListenPacket("udp", "localhost:34556")
	require.NoError(t, err)
	defer l1.Close()

	l2, err := net.ListenPacket("udp", "localhost:34557")
	require.NoError(t, err)
	defer l2.Close()

	inTH := &headers.Transport{
		Delivery:    ptrOf(headers.TransportDeliveryUnicast),
		Mode:        ptrOf(headers.TransportModeRecord),
		Protocol:    headers.TransportProtocolUDP,
		ClientPorts: &[2]int{34556, 34557},
	}

	res, th := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

	session := readSession(t, res)

	doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

	_, err = l1.WriteTo(mustMarshalPacketRTP(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 534,
			Timestamp:      54352,
			SSRC:           753621,
		},
		Payload: []byte{1, 2, 3, 4},
	}), &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: th.ServerPorts[0],
	})
	require.NoError(t, err)

	// wait for the packet's SSRC to be saved
	time.Sleep(200 * time.Millisecond)

	_, err = l2.WriteTo(mustMarshalPacketRTCP(&rtcp.SenderReport{
		SSRC:        753621,
		NTPTime:     ntp.Encode(time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)),
		RTPTime:     54352,
		PacketCount: 1,
		OctetCount:  4,
	}), &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: th.ServerPorts[1],
	})
	require.NoError(t, err)

	// skip firewall opening
	buf := make([]byte, 2048)
	_, _, err = l2.ReadFrom(buf)
	require.NoError(t, err)

	buf = make([]byte, 2048)
	n, _, err := l2.ReadFrom(buf)
	require.NoError(t, err)
	pkts, err := rtcp.Unmarshal(buf[:n])
	require.NoError(t, err)
	rr, ok := pkts[0].(*rtcp.ReceiverReport)
	require.True(t, ok)
	require.Equal(t, &rtcp.ReceiverReport{
		SSRC: rr.SSRC,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               rr.Reports[0].SSRC,
				LastSequenceNumber: 534,
				LastSenderReport:   4004511744,
				Delay:              rr.Reports[0].Delay,
			},
		},
		ProfileExtensions: []uint8{},
	}, rr)
}

func TestServerRecordTimeout(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
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
					onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				ReadTimeout:       1 * time.Second,
				RTSPAddress:       "localhost:8554",
				checkStreamPeriod: 500 * time.Millisecond,
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			medias := []*description.Media{testH264Media}

			doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

			inTH := &headers.Transport{
				Delivery: ptrOf(headers.TransportDeliveryUnicast),
				Mode:     ptrOf(headers.TransportModeRecord),
			}

			if transport == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, _ := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

			session := readSession(t, res)

			doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

			<-sessionClosed

			if transport == "tcp" {
				<-nconnClosed
			}
		})
	}
}

func TestServerRecordWithoutTeardown(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
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
					onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				ReadTimeout: 1 * time.Second,
				RTSPAddress: "localhost:8554",
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			medias := []*description.Media{testH264Media}

			doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

			inTH := &headers.Transport{
				Delivery: ptrOf(headers.TransportDeliveryUnicast),
				Mode:     ptrOf(headers.TransportModeRecord),
			}

			if transport == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, _ := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

			session := readSession(t, res)

			doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

			nconn.Close()

			<-sessionClosed
			<-nconnClosed
		})
	}
}

func TestServerRecordUDPChangeConn(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
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

	sxID := ""

	func() {
		var nconn net.Conn
		nconn, err = net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		medias := []*description.Media{testH264Media}

		doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

		inTH := &headers.Transport{
			Delivery:    ptrOf(headers.TransportDeliveryUnicast),
			Mode:        ptrOf(headers.TransportModeRecord),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, _ := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

		session := readSession(t, res)

		doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

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
				"CSeq":    base.HeaderValue{"1"},
				"Session": base.HeaderValue{sxID},
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)
	}()
}

func TestServerRecordDecodeErrors(t *testing.T) {
	for _, ca := range []struct {
		proto string
		name  string
	}{
		{"udp", "rtp invalid"},
		{"udp", "rtcp invalid"},
		{"udp", "rtp packets lost"},
		{"udp", "rtp unknown payload type"},
		{"udp", "wrong ssrc"},
		{"udp", "rtcp too big"},
		{"udp", "rtp too big"},
		{"tcp", "rtcp invalid"},
		{"tcp", "rtp packets lost"},
		{"tcp", "rtp unknown payload type"},
		{"tcp", "wrong ssrc"},
		{"tcp", "rtcp too big"},
	} {
		t.Run(ca.proto+" "+ca.name, func(t *testing.T) {
			errorRecv := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onPacketsLost: func(ctx *ServerHandlerOnPacketsLostCtx) {
						require.Equal(t, uint64(69), ctx.Lost)
						close(errorRecv)
					},
					onDecodeError: func(ctx *ServerHandlerOnDecodeErrorCtx) {
						switch {
						case ca.name == "rtp invalid":
							require.EqualError(t, ctx.Error, "RTP header size insufficient: 2 < 4")

						case ca.name == "rtcp invalid":
							require.EqualError(t, ctx.Error, "rtcp: packet too short")

						case ca.name == "rtp unknown payload type":
							require.EqualError(t, ctx.Error, "received RTP packet with unknown payload type: 111")

						case ca.name == "wrong ssrc":
							require.EqualError(t, ctx.Error, "received packet with wrong SSRC 456, expected 123")

						case ca.proto == "udp" && ca.name == "rtcp too big":
							require.EqualError(t, ctx.Error, "RTCP packet is too big to be read with UDP")

						case ca.proto == "tcp" && ca.name == "rtcp too big":
							require.EqualError(t, ctx.Error, "RTCP packet size (2000) is greater than maximum allowed (1472)")

						case ca.proto == "udp" && ca.name == "rtp too big":
							require.EqualError(t, ctx.Error, "RTP packet is too big to be read with UDP")

						default:
							t.Errorf("unexpected")
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
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			medias := []*description.Media{{
				Type: description.MediaTypeApplication,
				Formats: []format.Format{&format.Generic{
					PayloadTyp: 97,
					RTPMa:      "private/90000",
				}},
			}}

			doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

			inTH := &headers.Transport{
				Delivery: ptrOf(headers.TransportDeliveryUnicast),
				Mode:     ptrOf(headers.TransportModeRecord),
			}

			if ca.proto == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

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

			res, resTH := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

			session := readSession(t, res)

			doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

			var writeRTP func(buf []byte)
			var writeRTCP func(byts []byte)

			if ca.proto == "udp" { //nolint:dupl
				writeRTP = func(byts []byte) {
					_, err = l1.WriteTo(byts, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: resTH.ServerPorts[0],
					})
					require.NoError(t, err)
				}

				writeRTCP = func(byts []byte) {
					_, err = l2.WriteTo(byts, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: resTH.ServerPorts[1],
					})
					require.NoError(t, err)
				}
			} else {
				writeRTP = func(byts []byte) {
					err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
						Channel: 0,
						Payload: byts,
					}, make([]byte, 2048))
					require.NoError(t, err)
				}

				writeRTCP = func(byts []byte) {
					err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
						Channel: 1,
						Payload: byts,
					}, make([]byte, 2048))
					require.NoError(t, err)
				}
			}

			switch { //nolint:dupl
			case ca.name == "rtp invalid":
				writeRTP([]byte{0x01, 0x02})

			case ca.name == "rtcp invalid":
				writeRTCP([]byte{0x01, 0x02})

			case ca.name == "rtcp too big":
				writeRTCP(bytes.Repeat([]byte{0x01, 0x02}, 2000/2))

			case ca.name == "rtp packets lost":
				writeRTP(mustMarshalPacketRTP(&rtp.Packet{
					Header: rtp.Header{
						PayloadType:    97,
						SequenceNumber: 30,
					},
				}))

				writeRTP(mustMarshalPacketRTP(&rtp.Packet{
					Header: rtp.Header{
						PayloadType:    97,
						SequenceNumber: 100,
					},
				}))

			case ca.name == "rtp unknown payload type":
				writeRTP(mustMarshalPacketRTP(&rtp.Packet{
					Header: rtp.Header{
						PayloadType: 111,
					},
				}))

			case ca.name == "wrong ssrc":
				writeRTP(mustMarshalPacketRTP(&rtp.Packet{
					Header: rtp.Header{
						PayloadType:    97,
						SequenceNumber: 1,
						SSRC:           123,
					},
				}))

				writeRTP(mustMarshalPacketRTP(&rtp.Packet{
					Header: rtp.Header{
						PayloadType:    97,
						SequenceNumber: 2,
						SSRC:           456,
					},
				}))

			case ca.proto == "udp" && ca.name == "rtp too big":
				_, err = l1.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[0],
				})
				require.NoError(t, err)
			}

			<-errorRecv
		})
	}
}

func TestServerRecordPacketNTP(t *testing.T) {
	recv := make(chan struct{})
	first := false

	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
				ctx.Session.OnPacketRTPAny(func(medi *description.Media, _ format.Format, pkt *rtp.Packet) {
					if !first {
						first = true
					} else {
						ntp, ok := ctx.Session.PacketNTP(medi, pkt)
						require.Equal(t, true, ok)
						require.Equal(t, time.Date(2018, 2, 20, 19, 0, 1, 0, time.UTC), ntp.UTC())
						close(recv)
					}
				})

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
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	medias := []*description.Media{testH264Media}

	doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

	l1, err := net.ListenPacket("udp", "localhost:34556")
	require.NoError(t, err)
	defer l1.Close()

	l2, err := net.ListenPacket("udp", "localhost:34557")
	require.NoError(t, err)
	defer l2.Close()

	inTH := &headers.Transport{
		Delivery:    ptrOf(headers.TransportDeliveryUnicast),
		Mode:        ptrOf(headers.TransportModeRecord),
		Protocol:    headers.TransportProtocolUDP,
		ClientPorts: &[2]int{34556, 34557},
	}

	res, th := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

	session := readSession(t, res)

	doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

	_, err = l1.WriteTo(mustMarshalPacketRTP(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 534,
			Timestamp:      54352,
			SSRC:           753621,
		},
		Payload: []byte{1, 2, 3, 4},
	}), &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: th.ServerPorts[0],
	})
	require.NoError(t, err)

	// wait for the packet's SSRC to be saved
	time.Sleep(100 * time.Millisecond)

	_, err = l2.WriteTo(mustMarshalPacketRTCP(&rtcp.SenderReport{
		SSRC:        753621,
		NTPTime:     ntp.Encode(time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)),
		RTPTime:     54352,
		PacketCount: 1,
		OctetCount:  4,
	}), &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: th.ServerPorts[1],
	})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	_, err = l1.WriteTo(mustMarshalPacketRTP(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 535,
			Timestamp:      54352 + 90000,
			SSRC:           753621,
		},
		Payload: []byte{1, 2, 3, 4},
	}), &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: th.ServerPorts[0],
	})
	require.NoError(t, err)

	<-recv
}

func TestServerRecordPausePause(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
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
		RTSPAddress:    "localhost:8554",
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	medias := []*description.Media{{
		Type: description.MediaTypeApplication,
		Formats: []format.Format{&format.Generic{
			PayloadTyp: 97,
			RTPMa:      "private/90000",
		}},
	}}

	doAnnounce(t, conn, "rtsp://localhost:8554/teststream", medias)

	inTH := &headers.Transport{
		Delivery:    ptrOf(headers.TransportDeliveryUnicast),
		Mode:        ptrOf(headers.TransportModeRecord),
		Protocol:    headers.TransportProtocolUDP,
		ClientPorts: &[2]int{35466, 35467},
	}

	res, _ := doSetup(t, conn, "rtsp://localhost:8554/teststream/"+medias[0].Control, inTH, "")

	session := readSession(t, res)

	doRecord(t, conn, "rtsp://localhost:8554/teststream", session)

	doPause(t, conn, "rtsp://localhost:8554/teststream", session)

	doPause(t, conn, "rtsp://localhost:8554/teststream", session)
}
