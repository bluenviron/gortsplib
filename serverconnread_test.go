package gortsplib

import (
	"bufio"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

func TestServerConnReadReceivePackets(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			packetsReceived := make(chan struct{})

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

				onSetup := func(req *base.Request, th *headers.Transport, trackID int) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onPlay := func(req *base.Request) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onFrame := func(trackID int, typ StreamType, buf []byte) {
					require.Equal(t, 0, trackID)
					require.Equal(t, StreamTypeRTCP, typ)
					require.Equal(t, []byte("\x01\x02\x03\x04"), buf)
					close(packetsReceived)
				}

				err = <-conn.Read(ServerConnReadHandlers{
					OnSetup: onSetup,
					OnPlay:  onPlay,
					OnFrame: onFrame,
				})
				require.Equal(t, io.EOF, err)
			}()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			th := &headers.Transport{
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
				th.Protocol = StreamProtocolUDP
				th.ClientPorts = &[2]int{35466, 35467}
			} else {
				th.Protocol = StreamProtocolTCP
				th.InterleavedIds = &[2]int{0, 1}
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

			th, err = headers.ReadTransport(res.Header["Transport"])
			require.NoError(t, err)

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

			if proto == "udp" {
				time.Sleep(1 * time.Second)

				l1, err := net.ListenPacket("udp", "localhost:35467")
				require.NoError(t, err)
				defer l1.Close()

				l1.WriteTo([]byte("\x01\x02\x03\x04"), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
			} else {
				err = base.InterleavedFrame{
					TrackID:    0,
					StreamType: StreamTypeRTCP,
					Payload:    []byte("\x01\x02\x03\x04"),
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}

			<-packetsReceived
		})
	}
}

func TestServerConnReadWithoutSetupTrackID(t *testing.T) {
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

		onSetup := func(req *base.Request, th *headers.Transport, trackID int) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(req *base.Request) (*base.Response, error) {
			go func() {
				time.Sleep(100 * time.Millisecond)
				conn.WriteFrame(0, StreamTypeRTP, []byte("\x00\x00\x00\x00"))
			}()

			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		err = <-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
		})
		require.Equal(t, io.EOF, err)
	}()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
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
				InterleavedIds: &[2]int{0, 1},
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

func TestServerConnReadTCPResponseBeforeFrames(t *testing.T) {
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

		onSetup := func(req *base.Request, th *headers.Transport, trackID int) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(req *base.Request) (*base.Response, error) {
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

		err = <-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
		})
		require.Equal(t, io.EOF, err)
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
				InterleavedIds: &[2]int{0, 1},
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

func TestServerConnReadPlayMultiple(t *testing.T) {
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

		onSetup := func(req *base.Request, th *headers.Transport, trackID int) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(req *base.Request) (*base.Response, error) {
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

		err = <-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
		})
		require.Equal(t, io.EOF, err)
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
				InterleavedIds: &[2]int{0, 1},
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
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	buf := make([]byte, 2048)
	err = res.ReadIgnoreFrames(bconn.Reader, buf)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerConnReadPauseMultiple(t *testing.T) {
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

		onSetup := func(req *base.Request, th *headers.Transport, trackID int) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		onPlay := func(req *base.Request) (*base.Response, error) {
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

		onPause := func(req *base.Request) (*base.Response, error) {
			return &base.Response{
				StatusCode: base.StatusOK,
			}, nil
		}

		err = <-conn.Read(ServerConnReadHandlers{
			OnSetup: onSetup,
			OnPlay:  onPlay,
			OnPause: onPause,
		})
		require.Equal(t, io.EOF, err)
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
				InterleavedIds: &[2]int{0, 1},
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
