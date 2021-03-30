package gortsplib

import (
	"bufio"
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
			type pathTrackIDPair struct {
				path    string
				trackID int
			}
			setupDone := make(chan pathTrackIDPair)

			s, err := Serve("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := s.Accept()
				require.NoError(t, err)
				defer conn.Close()

				onAnnounce := func(ctx *ServerConnAnnounceCtx) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
					setupDone <- pathTrackIDPair{ctx.Path, ctx.TrackID}
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				<-conn.Read(ServerConnReadHandlers{
					OnAnnounce: onAnnounce,
					OnSetup:    onSetup,
				})
			}()

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

			err = base.Request{
				Method: base.Announce,
				URL:    base.MustParseURL("rtsp://localhost:8554/" + ca.path),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: byts,
			}.Write(bconn.Writer)
			require.NoError(t, err)

			var res base.Response
			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			th := &headers.Transport{
				Protocol: StreamProtocolTCP,
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

			err = base.Request{
				Method: base.Setup,
				URL:    base.MustParseURL(ca.url),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": th.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			pair := <-setupDone
			require.Equal(t, ca.path, pair.path)
			require.Equal(t, ca.trackID, pair.trackID)

			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerPublishSetupErrorDifferentPaths(t *testing.T) {
	serverErr := make(chan error)

	s, err := Serve("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := s.Accept()
		require.NoError(t, err)
		defer conn.Close()

		onAnnounce := func(ctx *ServerConnAnnounceCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		err = <-conn.Read(ServerConnReadHandlers{
			OnAnnounce: onAnnounce,
			OnSetup:    onSetup,
		})
		serverErr <- err
	}()

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

	err = base.Request{
		Method: base.Announce,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: StreamProtocolTCP,
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

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/test2stream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.Equal(t, "invalid track path (test2stream/trackID=0)", err.Error())
}

func TestServerPublishSetupErrorTrackTwice(t *testing.T) {
	serverErr := make(chan error)

	s, err := Serve("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := s.Accept()
		require.NoError(t, err)
		defer conn.Close()

		onAnnounce := func(ctx *ServerConnAnnounceCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		err = <-conn.Read(ServerConnReadHandlers{
			OnAnnounce: onAnnounce,
			OnSetup:    onSetup,
		})
		serverErr <- err
	}()

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

	err = base.Request{
		Method: base.Announce,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: StreamProtocolTCP,
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

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"3"},
			"Transport": th.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.Equal(t, "track 0 has already been setup", err.Error())
}

func TestServerPublishRecordErrorPartialTracks(t *testing.T) {
	serverErr := make(chan error)

	s, err := Serve("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := s.Accept()
		require.NoError(t, err)
		defer conn.Close()

		onAnnounce := func(ctx *ServerConnAnnounceCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onRecord := func(ctx *ServerConnRecordCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		err = <-conn.Read(ServerConnReadHandlers{
			OnAnnounce: onAnnounce,
			OnSetup:    onSetup,
			OnRecord:   onRecord,
		})
		serverErr <- err
	}()

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

	err = base.Request{
		Method: base.Announce,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th := &headers.Transport{
		Protocol: StreamProtocolTCP,
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

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": th.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Record,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.Equal(t, "not all announced tracks have been setup", err.Error())
}

func TestServerPublish(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			conf := ServerConf{}

			if proto == "udp" {
				conf.UDPRTPAddress = "127.0.0.1:8000"
				conf.UDPRTCPAddress = "127.0.0.1:8001"
			}

			s, err := conf.Serve("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := s.Accept()
				require.NoError(t, err)
				defer conn.Close()

				onAnnounce := func(ctx *ServerConnAnnounceCtx) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onRecord := func(ctx *ServerConnRecordCtx) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				rtpReceived := uint64(0)
				onFrame := func(trackID int, typ StreamType, buf []byte) {
					if atomic.SwapUint64(&rtpReceived, 1) == 0 {
						require.Equal(t, 0, trackID)
						require.Equal(t, StreamTypeRTP, typ)
						require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, buf)
					} else {
						require.Equal(t, 0, trackID)
						require.Equal(t, StreamTypeRTCP, typ)
						require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, buf)

						conn.WriteFrame(0, StreamTypeRTCP, []byte{0x09, 0x0A, 0x0B, 0x0C})
					}
				}

				<-conn.Read(ServerConnReadHandlers{
					OnAnnounce: onAnnounce,
					OnSetup:    onSetup,
					OnRecord:   onRecord,
					OnFrame:    onFrame,
				})
			}()

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

			err = base.Request{
				Method: base.Announce,
				URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Write(),
			}.Write(bconn.Writer)
			require.NoError(t, err)

			var res base.Response
			err = res.Read(bconn.Reader)
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
				inTH.Protocol = StreamProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = StreamProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			err = base.Request{
				Method: base.Setup,
				URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"2"},
					"Transport": inTH.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			err = res.Read(bconn.Reader)
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

			err = base.Request{
				Method: base.Record,
				URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq": base.HeaderValue{"3"},
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			// client -> server
			if proto == "udp" {
				time.Sleep(1 * time.Second)

				l1.WriteTo([]byte{0x01, 0x02, 0x03, 0x04}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[0],
				})

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
		})
	}
}

func TestServerPublishErrorWrongProtocol(t *testing.T) {
	conf := ServerConf{
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
	}

	s, err := conf.Serve("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := s.Accept()
		require.NoError(t, err)
		defer conn.Close()

		onAnnounce := func(ctx *ServerConnAnnounceCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onRecord := func(ctx *ServerConnRecordCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onFrame := func(trackID int, typ StreamType, buf []byte) {
			t.Error("should not happen")
		}

		<-conn.Read(ServerConnReadHandlers{
			OnAnnounce: onAnnounce,
			OnSetup:    onSetup,
			OnRecord:   onRecord,
			OnFrame:    onFrame,
		})
	}()

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

	err = base.Request{
		Method: base.Announce,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
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
		Protocol:    StreamProtocolUDP,
		ClientPorts: &[2]int{35466, 35467},
	}

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": inTH.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var th headers.Transport
	err = th.Read(res.Header["Transport"])
	require.NoError(t, err)

	err = base.Request{
		Method: base.Record,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
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
	conf := ServerConf{
		receiverReportPeriod: 1 * time.Second,
	}

	s, err := conf.Serve("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := s.Accept()
		require.NoError(t, err)
		defer conn.Close()

		onAnnounce := func(ctx *ServerConnAnnounceCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onRecord := func(ctx *ServerConnRecordCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onFrame := func(trackID int, typ StreamType, buf []byte) {
		}

		<-conn.Read(ServerConnReadHandlers{
			OnAnnounce: onAnnounce,
			OnSetup:    onSetup,
			OnRecord:   onRecord,
			OnFrame:    onFrame,
		})
	}()

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

	err = base.Request{
		Method: base.Announce,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
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
		Protocol:       StreamProtocolTCP,
		InterleavedIDs: &[2]int{0, 1},
	}

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"2"},
			"Transport": inTH.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var th headers.Transport
	err = th.Read(res.Header["Transport"])
	require.NoError(t, err)

	err = base.Request{
		Method: base.Record,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
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
