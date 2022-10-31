package gortsplib

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/conn"
	"github.com/aler9/gortsplib/pkg/headers"
)

func invalidURLAnnounceReq(t *testing.T, control string) base.Request {
	return base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: func() []byte {
			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}
			track.SetControl(control)

			sout := &psdp.SessionDescription{
				SessionName: psdp.SessionName("Stream"),
				Origin: psdp.Origin{
					Username:       "-",
					NetworkType:    "IN",
					AddressType:    "IP4",
					UnicastAddress: "127.0.0.1",
				},
				TimeDescriptions: []psdp.TimeDescription{
					{Timing: psdp.Timing{0, 0}}, //nolint:govet
				},
				MediaDescriptions: []*psdp.MediaDescription{
					track.MediaDescription(),
				},
			}

			byts, _ := sout.Marshal()
			return byts
		}(),
	}
}

func TestServerPublishErrorAnnounce(t *testing.T) {
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
			"invalid tracks",
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
			"invalid URL 1",
			invalidURLAnnounceReq(t, "rtsp://  aaaaa"),
			"unable to generate track URL",
		},
		{
			"invalid URL 2",
			invalidURLAnnounceReq(t, "rtsp://host"),
			"invalid track URL (rtsp://localhost:8554)",
		},
		{
			"invalid URL 3",
			invalidURLAnnounceReq(t, "rtsp://host/otherpath"),
			"invalid track path: must begin with 'teststream', but is 'otherpath'",
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
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
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

			_, err = writeReqReadRes(conn, ca.req)
			require.NoError(t, err)

			<-nconnClosed
		})
	}
}

func TestServerPublishSetupPath(t *testing.T) {
	for _, ca := range []struct {
		name    string
		control string
		url     string
		path    string
		trackID int
	}{
		{
			"normal",
			"trackID=0",
			"rtsp://localhost:8554/teststream/trackID=0",
			"teststream",
			0,
		},
		{
			"unordered id",
			"trackID=2",
			"rtsp://localhost:8554/teststream/trackID=2",
			"teststream",
			0,
		},
		{
			"custom param name",
			"testing=0",
			"rtsp://localhost:8554/teststream/testing=0",
			"teststream",
			0,
		},
		{
			"query",
			"?testing=0",
			"rtsp://localhost:8554/teststream?testing=0",
			"teststream",
			0,
		},
		{
			"subpath",
			"trackID=0",
			"rtsp://localhost:8554/test/stream/trackID=0",
			"test/stream",
			0,
		},
		{
			"subpath and query",
			"?testing=0",
			"rtsp://localhost:8554/test/stream?testing=0",
			"test/stream",
			0,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			s := &Server{
				Handler: &testServerHandler{
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						// make sure that track URLs are not overridden by NewServerStream()
						stream := NewServerStream(ctx.Tracks)
						defer stream.Close()

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						require.Equal(t, ca.path, ctx.Path)
						require.Equal(t, ca.trackID, ctx.TrackID)
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
			conn := conn.NewConn(nconn)

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}
			track.SetControl(ca.control)

			sout := &psdp.SessionDescription{
				SessionName: psdp.SessionName("Stream"),
				Origin: psdp.Origin{
					Username:       "-",
					NetworkType:    "IN",
					AddressType:    "IP4",
					UnicastAddress: "127.0.0.1",
				},
				TimeDescriptions: []psdp.TimeDescription{
					{Timing: psdp.Timing{0, 0}}, //nolint:govet
				},
				MediaDescriptions: []*psdp.MediaDescription{
					track.MediaDescription(),
				},
			}

			byts, _ := sout.Marshal()

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/" + ca.path),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: byts,
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			th := &headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL(ca.url),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": th.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerPublishErrorSetupDifferentPaths(t *testing.T) {
	serverErr := make(chan error)

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				serverErr <- ctx.Error
			},
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
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
	conn := conn.NewConn(nconn)

	track := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	tracks := Tracks{track}
	tracks.setControls()

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Marshal(false),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: headers.TransportProtocolTCP,
		Delivery: func() *headers.TransportDelivery {
			v := headers.TransportDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/test2stream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.EqualError(t, err, "invalid track path (test2stream/trackID=0)")
}

func TestServerPublishErrorSetupTrackTwice(t *testing.T) {
	serverErr := make(chan error)

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				serverErr <- ctx.Error
			},
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
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
	conn := conn.NewConn(nconn)

	track := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	tracks := Tracks{track}
	tracks.setControls()

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Marshal(false),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: headers.TransportProtocolTCP,
		Delivery: func() *headers.TransportDelivery {
			v := headers.TransportDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"3"},
			"Transport": th.Marshal(),
			"Session":   base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.EqualError(t, err, "track 0 has already been setup")
}

func TestServerPublishErrorRecordPartialTracks(t *testing.T) {
	serverErr := make(chan error)

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				serverErr <- ctx.Error
			},
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
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

	track1 := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	track2 := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	tracks := Tracks{track1, track2}
	tracks.setControls()

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Marshal(false),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: headers.TransportProtocolTCP,
		Delivery: func() *headers.TransportDelivery {
			v := headers.TransportDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Record,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.EqualError(t, err, "not all announced tracks have been setup")
}

func TestServerPublish(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
		"tls",
	} {
		t.Run(transport, func(t *testing.T) {
			nconnOpened := make(chan struct{})
			nconnClosed := make(chan struct{})
			sessionOpened := make(chan struct{})
			sessionClosed := make(chan struct{})

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
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						// send RTCP packets directly to the session.
						// these are sent after the response, only if onRecord returns StatusOK.
						ctx.Session.WritePacketRTCP(0, &testRTCPPacket)

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onPacketRTP: func(ctx *ServerHandlerOnPacketRTPCtx) {
						require.Equal(t, 0, ctx.TrackID)
						require.Equal(t, &testRTPPacket, ctx.Packet)
					},
					onPacketRTCP: func(ctx *ServerHandlerOnPacketRTCPCtx) {
						require.Equal(t, 0, ctx.TrackID)
						require.Equal(t, &testRTCPPacket, ctx.Packet)
						ctx.Session.WritePacketRTCP(0, &testRTCPPacket)
					},
				},
				RTSPAddress: "localhost:8554",
			}

			switch transport {
			case "udp":
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"

			case "tls":
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
				if transport == "tls" {
					return tls.Client(nconn, &tls.Config{InsecureSkipVerify: true})
				}
				return nconn
			}()
			conn := conn.NewConn(nconn)

			<-nconnOpened

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			tracks := Tracks{track}
			tracks.setControls()

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Marshal(false),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionOpened

			inTH := &headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
			}

			var l1 net.PacketConn
			var l2 net.PacketConn

			if transport == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}

				l1, err = net.ListenPacket("udp", "localhost:35466")
				require.NoError(t, err)
				defer l1.Close()

				l2, err = net.ListenPacket("udp", "localhost:35467")
				require.NoError(t, err)
				defer l2.Close()
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			var th headers.Transport
			err = th.Unmarshal(res.Header["Transport"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Record,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			// server -> client (direct)
			if transport == "udp" {
				buf := make([]byte, 2048)
				n, _, err := l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTCPPacketMarshaled, buf[:n])
			} else {
				f, err := conn.ReadInterleavedFrame()
				require.NoError(t, err)
				require.Equal(t, 1, f.Channel)
				require.Equal(t, testRTCPPacketMarshaled, f.Payload)
			}

			// skip firewall opening
			if transport == "udp" {
				buf := make([]byte, 2048)
				_, _, err := l2.ReadFrom(buf)
				require.NoError(t, err)
			}

			// client -> server
			if transport == "udp" {
				time.Sleep(1 * time.Second)

				l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[0],
				})

				time.Sleep(500 * time.Millisecond)

				l2.WriteTo(testRTCPPacketMarshaled, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
			} else {
				err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 0,
					Payload: testRTPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err)

				err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 1,
					Payload: testRTCPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err)
			}

			// server -> client (RTCP)
			if transport == "udp" {
				buf := make([]byte, 2048)
				n, _, err := l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, testRTCPPacketMarshaled, buf[:n])
			} else {
				f, err := conn.ReadInterleavedFrame()
				require.NoError(t, err)
				require.Equal(t, 1, f.Channel)
				require.Equal(t, testRTCPPacketMarshaled, f.Payload)
			}

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

			<-sessionClosed

			nconn.Close()
			<-nconnClosed
		})
	}
}

func TestServerPublishErrorInvalidProtocol(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPacketRTP: func(ctx *ServerHandlerOnPacketRTPCtx) {
				t.Error("should not happen")
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

	track := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	tracks := Tracks{track}
	tracks.setControls()

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Marshal(false),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	inTH := &headers.Transport{
		Delivery: func() *headers.TransportDelivery {
			v := headers.TransportDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		Protocol:    headers.TransportProtocolUDP,
		ClientPorts: &[2]int{35466, 35467},
	}

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": inTH.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	var th headers.Transport
	err = th.Unmarshal(res.Header["Transport"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Record,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
		Channel: 0,
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}, make([]byte, 1024))
	require.NoError(t, err)
}

func TestServerPublishRTCPReport(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		udpReceiverReportPeriod: 1 * time.Second,
		UDPRTPAddress:           "127.0.0.1:8000",
		UDPRTCPAddress:          "127.0.0.1:8001",
		RTSPAddress:             "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	track := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	tracks := Tracks{track}
	tracks.setControls()

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Marshal(false),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	l1, err := net.ListenPacket("udp", "localhost:34556")
	require.NoError(t, err)
	defer l1.Close()

	l2, err := net.ListenPacket("udp", "localhost:34557")
	require.NoError(t, err)
	defer l2.Close()

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
			"Transport": headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
				Protocol:    headers.TransportProtocolUDP,
				ClientPorts: &[2]int{34556, 34557},
			}.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Unmarshal(res.Header["Session"])
	require.NoError(t, err)

	var th headers.Transport
	err = th.Unmarshal(res.Header["Transport"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Record,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	byts, _ := (&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 534,
			Timestamp:      54352,
			SSRC:           753621,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}).Marshal()
	_, err = l1.WriteTo(byts, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: th.ServerPorts[0],
	})
	require.NoError(t, err)

	byts, _ = (&rtcp.SenderReport{
		SSRC:        753621,
		NTPTime:     0xcbddcc34999997ff,
		RTPTime:     54352,
		PacketCount: 1,
		OctetCount:  4,
	}).Marshal()
	_, err = l2.WriteTo(byts, &net.UDPAddr{
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
	pkt, err := rtcp.Unmarshal(buf[:n])
	require.NoError(t, err)
	rr, ok := pkt[0].(*rtcp.ReceiverReport)
	require.True(t, ok)
	require.Equal(t, &rtcp.ReceiverReport{
		SSRC: rr.SSRC,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               rr.Reports[0].SSRC,
				LastSequenceNumber: 534,
				LastSenderReport:   rr.Reports[0].LastSenderReport,
				Delay:              rr.Reports[0].Delay,
			},
		},
		ProfileExtensions: []uint8{},
	}, rr)
}

func TestServerPublishTimeout(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			nconnClosed := make(chan struct{})
			sessionClosed := make(chan struct{})

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
						}, nil, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
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
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			tracks := Tracks{track}
			tracks.setControls()

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Marshal(false),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			inTH := &headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
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

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			var th headers.Transport
			err = th.Unmarshal(res.Header["Transport"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Record,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionClosed

			if transport == "tcp" {
				<-nconnClosed
			}
		})
	}
}

func TestServerPublishWithoutTeardown(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			nconnClosed := make(chan struct{})
			sessionClosed := make(chan struct{})

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
						}, nil, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
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
			conn := conn.NewConn(nconn)

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			tracks := Tracks{track}
			tracks.setControls()

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Marshal(false),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			inTH := &headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
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

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			var th headers.Transport
			err = th.Unmarshal(res.Header["Transport"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Record,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
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

func TestServerPublishUDPChangeConn(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
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

		track := &TrackH264{
			PayloadType: 96,
			SPS:         []byte{0x01, 0x02, 0x03, 0x04},
			PPS:         []byte{0x01, 0x02, 0x03, 0x04},
		}

		tracks := Tracks{track}
		tracks.setControls()

		res, err := writeReqReadRes(conn, base.Request{
			Method: base.Announce,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":         base.HeaderValue{"1"},
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: tracks.Marshal(false),
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModeRecord
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, err = writeReqReadRes(conn, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"2"},
				"Transport": inTH.Marshal(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		var sx headers.Session
		err = sx.Unmarshal(res.Header["Session"])
		require.NoError(t, err)

		res, err = writeReqReadRes(conn, base.Request{
			Method: base.Record,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"3"},
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

func TestServerPublishDecodeErrors(t *testing.T) {
	for _, ca := range []string{
		"invalid rtp",
		"invalid rtcp",
	} {
		t.Run(ca, func(t *testing.T) {
			errorRecv := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onDecodeError: func(ctx *ServerHandlerOnDecodeErrorCtx) {
						switch ca {
						case "invalid rtp":
							require.EqualError(t, ctx.Error, "RTP header size insufficient: 2 < 4")
						case "invalid rtcp":
							require.EqualError(t, ctx.Error, "rtcp: packet too short")
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

			tracks := Tracks{&TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}}
			tracks.setControls()

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Marshal(false),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			inTH := &headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
				Protocol:    headers.TransportProtocolUDP,
				ClientPorts: &[2]int{35466, 35467},
			}

			l1, err := net.ListenPacket("udp", "127.0.0.1:35466")
			require.NoError(t, err)
			defer l1.Close()

			l2, err := net.ListenPacket("udp", "127.0.0.1:35467")
			require.NoError(t, err)
			defer l2.Close()

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var sx headers.Session
			err = sx.Unmarshal(res.Header["Session"])
			require.NoError(t, err)

			var resTH headers.Transport
			err = resTH.Unmarshal(res.Header["Transport"])
			require.NoError(t, err)

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Record,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": base.HeaderValue{sx.Session},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			switch ca {
			case "invalid rtp":
				l1.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[0],
				})

			case "invalid rtcp":
				l2.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: resTH.ServerPorts[1],
				})
			}

			<-errorRecv
		})
	}
}
