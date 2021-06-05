package gortsplib

import (
	"bufio"
	"crypto/tls"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

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
			"no tracks",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: func() []byte {
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
						MediaDescriptions: []*psdp.MediaDescription{},
					}

					byts, _ := sout.Marshal()
					return byts
				}(),
			},
			"no tracks defined in the SDP",
		},
		{
			"invalid URL 1",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: func() []byte {
					track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
					require.NoError(t, err)
					track.Media.Attributes = append(track.Media.Attributes, psdp.Attribute{
						Key:   "control",
						Value: "rtsp://  aaaaa",
					})

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
							track.Media,
						},
					}

					byts, _ := sout.Marshal()
					return byts
				}(),
			},
			"unable to generate track URL",
		},
		{
			"invalid URL 2",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: func() []byte {
					track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
					require.NoError(t, err)
					track.Media.Attributes = append(track.Media.Attributes, psdp.Attribute{
						Key:   "control",
						Value: "rtsp://host",
					})

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
							track.Media,
						},
					}

					byts, _ := sout.Marshal()
					return byts
				}(),
			},
			"invalid track URL (rtsp://localhost:8554)",
		},
		{
			"invalid URL 3",
			base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: func() []byte {
					track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
					require.NoError(t, err)
					track.Media.Attributes = append(track.Media.Attributes, psdp.Attribute{
						Key:   "control",
						Value: "rtsp://host/otherpath",
					})

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
							track.Media,
						},
					}

					byts, _ := sout.Marshal()
					return byts
				}(),
			},
			"invalid track path: must begin with 'teststream', but is 'otherpath'",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			connClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						require.Equal(t, ca.err, ctx.Error.Error())
						close(connClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			_, err = writeReqReadRes(bconn, ca.req)
			require.NoError(t, err)

			<-connClosed
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
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
						require.Equal(t, ca.path, ctx.Path)
						require.Equal(t, ca.trackID, ctx.TrackID)
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
				},
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)
			track.Media.Attributes = append(track.Media.Attributes, psdp.Attribute{
				Key:   "control",
				Value: ca.control,
			})

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
					track.Media,
				},
			}

			byts, _ := sout.Marshal()

			res, err := writeReqReadRes(bconn, base.Request{
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
				Protocol: base.StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL(ca.url),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": th.Write(),
					"Session":   res.Header["Session"],
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
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
		},
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	tracks := Tracks{track}
	for i, t := range tracks {
		t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})
	}

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: base.StreamProtocolTCP,
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/test2stream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Write(),
			"Session":   res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.Equal(t, "invalid track path (test2stream/trackID=0)", err.Error())
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
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
		},
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	tracks := Tracks{track}
	for i, t := range tracks {
		t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})
	}

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: base.StreamProtocolTCP,
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Write(),
			"Session":   res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"3"},
			"Transport": th.Write(),
			"Session":   res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.Equal(t, "track 0 has already been setup", err.Error())
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
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
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
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	track1, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	track2, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	tracks := Tracks{track1, track2}
	for i, t := range tracks {
		t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})
	}

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: base.StreamProtocolTCP,
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Write(),
			"Session":   res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Record,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.Equal(t, "not all announced tracks have been setup", err.Error())
}

func TestServerPublish(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
		"tls",
	} {
		t.Run(proto, func(t *testing.T) {
			connOpened := make(chan struct{})
			connClosed := make(chan struct{})
			sessionOpened := make(chan struct{})
			sessionClosed := make(chan struct{})
			rtpReceived := uint64(0)

			s := &Server{
				Handler: &testServerHandler{
					onConnOpen: func(ctx *ServerHandlerOnConnOpenCtx) {
						close(connOpened)
					},
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						close(connClosed)
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
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onFrame: func(ctx *ServerHandlerOnFrameCtx) {
						if atomic.SwapUint64(&rtpReceived, 1) == 0 {
							require.Equal(t, 0, ctx.TrackID)
							require.Equal(t, StreamTypeRTP, ctx.StreamType)
							require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, ctx.Payload)
						} else {
							require.Equal(t, 0, ctx.TrackID)
							require.Equal(t, StreamTypeRTCP, ctx.StreamType)
							require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, ctx.Payload)

							ctx.Session.WriteFrame(0, StreamTypeRTCP, []byte{0x09, 0x0A, 0x0B, 0x0C})
						}
					},
				},
			}

			switch proto {
			case "udp":
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"

			case "tls":
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()

			conn := func() net.Conn {
				if proto == "tls" {
					return tls.Client(nconn, &tls.Config{InsecureSkipVerify: true})
				}
				return nconn
			}()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			<-connOpened

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			tracks := Tracks{track}
			for i, t := range tracks {
				t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
					Key:   "control",
					Value: "trackID=" + strconv.FormatInt(int64(i), 10),
				})
			}

			res, err := writeReqReadRes(bconn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Write(),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionOpened

			inTH := &headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
			}

			if proto == "udp" {
				inTH.Protocol = base.StreamProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = base.StreamProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Write(),
					"Session":   res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var th headers.Transport
			err = th.Read(res.Header["Transport"])
			require.NoError(t, err)

			var l1 net.PacketConn
			var l2 net.PacketConn
			if proto == "udp" {
				l1, err = net.ListenPacket("udp", "localhost:35466")
				require.NoError(t, err)
				defer l1.Close()

				l2, err = net.ListenPacket("udp", "localhost:35467")
				require.NoError(t, err)
				defer l2.Close()
			}

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Record,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			// client -> server
			if proto == "udp" {
				time.Sleep(1 * time.Second)

				l1.WriteTo([]byte{0x01, 0x02, 0x03, 0x04}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[0],
				})

				time.Sleep(500 * time.Millisecond)

				l2.WriteTo([]byte{0x05, 0x06, 0x07, 0x08}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})

			} else {
				err = base.InterleavedFrame{
					TrackID:    0,
					StreamType: StreamTypeRTP,
					Payload:    []byte{0x01, 0x02, 0x03, 0x04},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = base.InterleavedFrame{
					TrackID:    0,
					StreamType: StreamTypeRTCP,
					Payload:    []byte{0x05, 0x06, 0x07, 0x08},
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}

			// server -> client (RTCP)
			if proto == "udp" {
				// skip firewall opening
				buf := make([]byte, 2048)
				_, _, err := l2.ReadFrom(buf)
				require.NoError(t, err)

				buf = make([]byte, 2048)
				n, _, err := l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, []byte{0x09, 0x0A, 0x0B, 0x0C}, buf[:n])

			} else {
				var f base.InterleavedFrame
				f.Payload = make([]byte, 2048)
				err := f.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, StreamTypeRTCP, f.StreamType)
				require.Equal(t, []byte{0x09, 0x0A, 0x0B, 0x0C}, f.Payload)
			}

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Teardown,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionClosed

			conn.Close()
			<-connClosed
		})
	}
}

func TestServerPublishErrorWrongProtocol(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
			onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onFrame: func(ctx *ServerHandlerOnFrameCtx) {
				t.Error("should not happen")
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	tracks := Tracks{track}
	for i, t := range tracks {
		t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})
	}

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	inTH := &headers.Transport{
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		Protocol:    base.StreamProtocolUDP,
		ClientPorts: &[2]int{35466, 35467},
	}

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": inTH.Write(),
			"Session":   res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var th headers.Transport
	err = th.Read(res.Header["Transport"])
	require.NoError(t, err)

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Record,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.InterleavedFrame{
		TrackID:    0,
		StreamType: StreamTypeRTP,
		Payload:    []byte{0x01, 0x02, 0x03, 0x04},
	}.Write(bconn.Writer)
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
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
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
		receiverReportPeriod: 1 * time.Second,
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	tracks := Tracks{track}
	for i, t := range tracks {
		t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})
	}

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	inTH := &headers.Transport{
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModeRecord
			return &v
		}(),
		Protocol:       base.StreamProtocolTCP,
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": inTH.Write(),
			"Session":   res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var th headers.Transport
	err = th.Read(res.Header["Transport"])
	require.NoError(t, err)

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Record,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": res.Header["Session"],
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
	err = base.InterleavedFrame{
		TrackID:    0,
		StreamType: StreamTypeRTP,
		Payload:    byts,
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var f base.InterleavedFrame
	f.Payload = make([]byte, 2048)
	f.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, StreamTypeRTCP, f.StreamType)
	pkt, err := rtcp.Unmarshal(f.Payload)
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

	err = base.InterleavedFrame{
		TrackID:    0,
		StreamType: StreamTypeRTP,
		Payload:    byts,
	}.Write(bconn.Writer)
	require.NoError(t, err)
}

func TestServerPublishTimeout(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			connClosed := make(chan struct{})
			sessionClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						close(connClosed)
					},
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
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
			}

			if proto == "udp" {
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(nconn), bufio.NewWriter(nconn))

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			tracks := Tracks{track}
			for i, t := range tracks {
				t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
					Key:   "control",
					Value: "trackID=" + strconv.FormatInt(int64(i), 10),
				})
			}

			res, err := writeReqReadRes(bconn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Write(),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			inTH := &headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
			}

			if proto == "udp" {
				inTH.Protocol = base.StreamProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = base.StreamProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Write(),
					"Session":   res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var th headers.Transport
			err = th.Read(res.Header["Transport"])
			require.NoError(t, err)

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Record,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionClosed

			if proto == "tcp" {
				<-connClosed
			}
		})
	}
}

func TestServerPublishWithoutTeardown(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			connClosed := make(chan struct{})
			sessionClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						close(connClosed)
					},
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
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
			}

			if proto == "udp" {
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			bconn := bufio.NewReadWriter(bufio.NewReader(nconn), bufio.NewWriter(nconn))

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			tracks := Tracks{track}
			for i, t := range tracks {
				t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
					Key:   "control",
					Value: "trackID=" + strconv.FormatInt(int64(i), 10),
				})
			}

			res, err := writeReqReadRes(bconn, base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Write(),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			inTH := &headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
					return &v
				}(),
			}

			if proto == "udp" {
				inTH.Protocol = base.StreamProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = base.StreamProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Write(),
					"Session":   res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var th headers.Transport
			err = th.Read(res.Header["Transport"])
			require.NoError(t, err)

			res, err = writeReqReadRes(bconn, base.Request{
				Method: base.Record,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			nconn.Close()

			<-sessionClosed
			<-connClosed
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
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *uint32, error) {
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
			onFrame: func(ctx *ServerHandlerOnFrameCtx) {
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	sxID := ""

	func() {
		conn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer conn.Close()
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)

		tracks := Tracks{track}
		for i, t := range tracks {
			t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
				Key:   "control",
				Value: "trackID=" + strconv.FormatInt(int64(i), 10),
			})
		}

		res, err := writeReqReadRes(bconn, base.Request{
			Method: base.Announce,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":         base.HeaderValue{"1"},
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: tracks.Write(),
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		inTH := &headers.Transport{
			Delivery: func() *base.StreamDelivery {
				v := base.StreamDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModeRecord
				return &v
			}(),
			Protocol:    base.StreamProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, err = writeReqReadRes(bconn, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"2"},
				"Transport": inTH.Write(),
				"Session":   res.Header["Session"],
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		res, err = writeReqReadRes(bconn, base.Request{
			Method: base.Record,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"3"},
				"Session": res.Header["Session"],
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)
		sxID = res.Header["Session"][0]
	}()

	func() {
		conn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer conn.Close()
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		res, err := writeReqReadRes(bconn, base.Request{
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
