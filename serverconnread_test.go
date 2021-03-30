package gortsplib

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

func TestServerReadSetupPath(t *testing.T) {
	for _, ca := range []struct {
		name    string
		url     string
		path    string
		trackID int
	}{
		{
			"normal",
			"rtsp://localhost:8554/teststream/trackID=2",
			"teststream",
			2,
		},
		{
			"with query",
			"rtsp://localhost:8554/teststream?testing=123/trackID=4",
			"teststream",
			4,
		},
		{
			// this is needed to support reading mpegts with ffmpeg
			"without track id",
			"rtsp://localhost:8554/teststream/",
			"teststream",
			0,
		},
		{
			"subpath",
			"rtsp://localhost:8554/test/stream/trackID=0",
			"test/stream",
			0,
		},
		{
			"subpath without track id",
			"rtsp://localhost:8554/test/stream/",
			"test/stream",
			0,
		},
		{
			"subpath with query",
			"rtsp://localhost:8554/test/stream?testing=123/trackID=4",
			"test/stream",
			4,
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

				onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
					setupDone <- pathTrackIDPair{ctx.Path, ctx.TrackID}
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				<-conn.Read(ServerConnReadHandlers{
					OnSetup: onSetup,
				})
			}()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			th := &headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{ca.trackID * 2, (ca.trackID * 2) + 1},
			}

			err = base.Request{
				Method: base.Setup,
				URL:    base.MustParseURL(ca.url),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": th.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			pair := <-setupDone
			require.Equal(t, ca.path, pair.path)
			require.Equal(t, ca.trackID, pair.trackID)

			var res base.Response
			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerReadSetupErrorDifferentPaths(t *testing.T) {
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

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		err = <-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
		})
		serverErr <- err
	}()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	th := &headers.Transport{
		Protocol: StreamProtocolTCP,
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModePlay
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"1"},
			"Transport": th.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th.InterleavedIDs = &[2]int{2, 3}

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/test12stream/trackID=1"),
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
	require.Equal(t, "can't setup tracks with different paths", err.Error())
}

func TestServerReadSetupErrorTrackTwice(t *testing.T) {
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

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		err = <-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
		})
		serverErr <- err
	}()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	th := &headers.Transport{
		Protocol: StreamProtocolTCP,
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModePlay
			return &v
		}(),
		InterleavedIDs: &[2]int{0, 1},
	}

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"1"},
			"Transport": th.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	th.InterleavedIDs = &[2]int{2, 3}

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
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	err = <-serverErr
	require.Equal(t, "track 0 has already been setup", err.Error())
}

func TestServerRead(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			framesReceived := make(chan struct{})

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

				onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onPlay := func(ctx *ServerConnPlayCtx) (*base.Response, error) {
					conn.WriteFrame(0, StreamTypeRTP, []byte{0x01, 0x02, 0x03, 0x04})
					conn.WriteFrame(0, StreamTypeRTCP, []byte{0x05, 0x06, 0x07, 0x08})

					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onFrame := func(trackID int, typ StreamType, buf []byte) {
					require.Equal(t, 0, trackID)
					require.Equal(t, StreamTypeRTCP, typ)
					require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, buf)
					close(framesReceived)
				}

				<-conn.Read(ServerConnReadHandlers{
					OnSetup: onSetup,
					OnPlay:  onPlay,
					OnFrame: onFrame,
				})
			}()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			inTH := &headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
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
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			var res base.Response
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
				Method: base.Play,
				URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq": base.HeaderValue{"2"},
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			// server -> client
			if proto == "udp" {
				buf := make([]byte, 2048)
				n, _, err := l1.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, buf[:n])

				buf = make([]byte, 2048)
				n, _, err = l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, buf[:n])

			} else {
				var f base.InterleavedFrame
				f.Payload = make([]byte, 2048)
				err := f.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, StreamTypeRTP, f.StreamType)
				require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, f.Payload)

				f.Payload = make([]byte, 2048)
				err = f.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, StreamTypeRTCP, f.StreamType)
				require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, f.Payload)
			}

			// client -> server (RTCP)
			if proto == "udp" {
				l2.WriteTo([]byte{0x01, 0x02, 0x03, 0x04}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
			} else {
				err = base.InterleavedFrame{
					TrackID:    0,
					StreamType: StreamTypeRTCP,
					Payload:    []byte{0x01, 0x02, 0x03, 0x04},
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}

			<-framesReceived
		})
	}
}

func TestServerReadTCPResponseBeforeFrames(t *testing.T) {
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

		writerDone := make(chan struct{})
		defer func() { <-writerDone }()
		writerTerminate := make(chan struct{})
		defer close(writerTerminate)

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(ctx *ServerConnPlayCtx) (*base.Response, error) {
			go func() {
				defer close(writerDone)

				conn.WriteFrame(0, StreamTypeRTP, []byte("\x00\x00\x00\x00"))

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for {
					select {
					case <-t.C:
						conn.WriteFrame(0, StreamTypeRTP, []byte("\x00\x00\x00\x00"))
					case <-writerTerminate:
						return
					}
				}
			}()

			time.Sleep(50 * time.Millisecond)

			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		<-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
		})
	}()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var fr base.InterleavedFrame
	fr.Payload = make([]byte, 2048)
	err = fr.Read(bconn.Reader)
	require.NoError(t, err)
}

func TestServerReadPlayPlay(t *testing.T) {
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

		writerDone := make(chan struct{})
		defer func() { <-writerDone }()
		writerTerminate := make(chan struct{})
		defer close(writerTerminate)

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(ctx *ServerConnPlayCtx) (*base.Response, error) {
			if conn.State() != ServerConnStatePlay {
				go func() {
					defer close(writerDone)

					t := time.NewTicker(50 * time.Millisecond)
					defer t.Stop()

					for {
						select {
						case <-t.C:
							conn.WriteFrame(0, StreamTypeRTP, []byte("\x00\x00\x00\x00"))
						case <-writerTerminate:
							return
						}
					}
				}()
			}

			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		<-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
		})
	}()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	buf := make([]byte, 2048)
	err = res.ReadIgnoreFrames(bconn.Reader, buf)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerReadPlayPausePlay(t *testing.T) {
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

		writerStarted := false
		writerDone := make(chan struct{})
		defer func() { <-writerDone }()
		writerTerminate := make(chan struct{})
		defer close(writerTerminate)

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(ctx *ServerConnPlayCtx) (*base.Response, error) {
			if !writerStarted {
				writerStarted = true
				go func() {
					defer close(writerDone)

					t := time.NewTicker(50 * time.Millisecond)
					defer t.Stop()

					for {
						select {
						case <-t.C:
							conn.WriteFrame(0, StreamTypeRTP, []byte("\x00\x00\x00\x00"))
						case <-writerTerminate:
							return
						}
					}
				}()
			}

			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPause := func(ctx *ServerConnPauseCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		<-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
			OnPause: onPause,
		})
	}()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Pause,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	buf := make([]byte, 2048)
	err = res.ReadIgnoreFrames(bconn.Reader, buf)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var fr base.InterleavedFrame
	fr.Payload = make([]byte, 2048)
	err = fr.Read(bconn.Reader)
	require.NoError(t, err)
}

func TestServerReadPlayPausePause(t *testing.T) {
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

		writerDone := make(chan struct{})
		defer func() { <-writerDone }()
		writerTerminate := make(chan struct{})
		defer close(writerTerminate)

		onSetup := func(ctx *ServerConnSetupCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(ctx *ServerConnPlayCtx) (*base.Response, error) {
			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for {
					select {
					case <-t.C:
						conn.WriteFrame(0, StreamTypeRTP, []byte("\x00\x00\x00\x00"))
					case <-writerTerminate:
						return
					}
				}
			}()

			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPause := func(ctx *ServerConnPauseCtx) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		<-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
			OnPause: onPause,
		})
	}()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Pause,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	buf := make([]byte, 2048)
	err = res.ReadIgnoreFrames(bconn.Reader, buf)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Pause,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	buf = make([]byte, 2048)
	err = res.ReadIgnoreFrames(bconn.Reader, buf)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}
