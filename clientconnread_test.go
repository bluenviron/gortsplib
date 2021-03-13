package gortsplib

import (
	"bufio"
	"crypto/tls"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

func TestClientConnRead(t *testing.T) {
	for _, ca := range []struct {
		encrypted bool
		proto     string
	}{
		{false, "udp"},
		{false, "tcp"},
		{true, "tcp"},
	} {
		encryptedStr := func() string {
			if ca.encrypted {
				return "encrypted"
			}
			return "plain"
		}()

		t.Run(encryptedStr+"_"+ca.proto, func(t *testing.T) {
			var scheme string
			var l net.Listener
			if ca.encrypted {
				scheme = "rtsps"

				li, err := net.Listen("tcp", "localhost:8554")
				require.NoError(t, err)
				defer li.Close()

				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)

				l = tls.NewListener(li, &tls.Config{Certificates: []tls.Certificate{cert}})

			} else {
				scheme = "rtsp"

				var err error
				l, err = net.Listen("tcp", "localhost:8554")
				require.NoError(t, err)
				defer l.Close()
			}

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				var req base.Request
				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)
				sdp := Tracks{track}.Write()

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
					},
					Body: sdp,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				th, err := headers.ReadTransport(req.Header["Transport"])
				require.NoError(t, err)

				if ca.proto == "udp" {
					err = base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Transport": headers.Transport{
								Protocol: StreamProtocolUDP,
								Delivery: func() *base.StreamDelivery {
									v := base.StreamDeliveryUnicast
									return &v
								}(),
								ClientPorts: th.ClientPorts,
								ServerPorts: &[2]int{34556, 34557},
							}.Write(),
						},
					}.Write(bconn.Writer)
					require.NoError(t, err)

				} else {
					err = base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Transport": headers.Transport{
								Protocol: StreamProtocolTCP,
								Delivery: func() *base.StreamDelivery {
									v := base.StreamDeliveryUnicast
									return &v
								}(),
								ClientPorts:    th.ClientPorts,
								InterleavedIds: &[2]int{0, 1},
							}.Write(),
						},
					}.Write(bconn.Writer)
					require.NoError(t, err)
				}

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				if ca.proto == "udp" {
					time.Sleep(1 * time.Second)

					l1, err := net.ListenPacket("udp", "localhost:34556")
					require.NoError(t, err)
					defer l1.Close()

					l1.WriteTo([]byte("\x00\x00\x00\x00"), &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})

				} else {
					err = base.InterleavedFrame{
						TrackID:    0,
						StreamType: StreamTypeRTP,
						Payload:    []byte("\x00\x00\x00\x00"),
					}.Write(bconn.Writer)
					require.NoError(t, err)
				}
			}()

			conf := ClientConf{
				StreamProtocol: func() *StreamProtocol {
					if ca.proto == "udp" {
						v := StreamProtocolUDP
						return &v
					}
					v := StreamProtocolTCP
					return &v
				}(),
			}

			conn, err := conf.DialRead(scheme + "://localhost:8554/teststream")
			require.NoError(t, err)

			frameRecv := make(chan struct{})
			done := conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
				close(frameRecv)
			})

			<-frameRecv
			conn.Close()
			<-done

			done = conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
				t.Error("should not happen")
			})
			<-done
		})
	}
}

func TestClientConnReadNoServerPorts(t *testing.T) {
	for _, ca := range []string{
		"zero",
		"no",
	} {
		t.Run(ca, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				var req base.Request
				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)
				sdp := Tracks{track}.Write()

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
					},
					Body: sdp,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				th, err := headers.ReadTransport(req.Header["Transport"])
				require.NoError(t, err)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": headers.Transport{
							Protocol: StreamProtocolUDP,
							Delivery: func() *base.StreamDelivery {
								v := base.StreamDeliveryUnicast
								return &v
							}(),
							ClientPorts: th.ClientPorts,
							ServerPorts: func() *[2]int {
								if ca == "zero" {
									return &[2]int{0, 0}
								}
								return nil
							}(),
						}.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				time.Sleep(1 * time.Second)

				l1, err := net.ListenPacket("udp", "localhost:0")
				require.NoError(t, err)
				defer l1.Close()

				l1.WriteTo([]byte("\x00\x00\x00\x00"), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ClientPorts[0],
				})
			}()

			conf := ClientConf{
				AnyPortEnable: true,
			}

			conn, err := conf.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			frameRecv := make(chan struct{})
			done := conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
				close(frameRecv)
			})

			<-frameRecv
			conn.Close()
			<-done
		})
	}
}

func TestClientConnReadAutomaticProtocol(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		var req base.Request
		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)
		sdp := Tracks{track}.Write()

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: sdp,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		err = base.Response{
			StatusCode: base.StatusUnsupportedTransport,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol: StreamProtocolTCP,
					Delivery: func() *base.StreamDelivery {
						v := base.StreamDeliveryUnicast
						return &v
					}(),
					InterleavedIds: &[2]int{0, 1},
				}.Write(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = base.InterleavedFrame{
			TrackID:    0,
			StreamType: StreamTypeRTP,
			Payload:    []byte("\x00\x00\x00\x00"),
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	conf := ClientConf{StreamProtocol: nil}

	conn, err := conf.DialRead("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	frameRecv := make(chan struct{})
	done := conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
		close(frameRecv)
	})

	<-frameRecv
	conn.Close()
	<-done
}

func TestClientConnReadRedirect(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		var req base.Request
		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		err = base.Response{
			StatusCode: base.StatusMovedPermanently,
			Header: base.Header{
				"Location": base.HeaderValue{"rtsp://localhost:8554/test"},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		conn.Close()

		conn, err = l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		bconn = bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)
		sdp := Tracks{track}.Write()

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: sdp,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		th, err := headers.ReadTransport(req.Header["Transport"])
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol: StreamProtocolUDP,
					Delivery: func() *base.StreamDelivery {
						v := base.StreamDeliveryUnicast
						return &v
					}(),
					ClientPorts: th.ClientPorts,
					ServerPorts: &[2]int{34556, 34557},
				}.Write(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		time.Sleep(1 * time.Second)

		l1, err := net.ListenPacket("udp", "localhost:34556")
		require.NoError(t, err)
		defer l1.Close()

		l1.WriteTo([]byte("\x00\x00\x00\x00"), &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: th.ClientPorts[0],
		})
	}()

	conn, err := DialRead("rtsp://localhost:8554/path1")
	require.NoError(t, err)

	frameRecv := make(chan struct{})
	done := conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
		close(frameRecv)
	})

	<-frameRecv
	conn.Close()
	<-done
}

func TestClientConnReadPause(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
			require.NoError(t, err)
			defer cnt1.close()

			time.Sleep(1 * time.Second)

			cnt2, err := newContainer("ffmpeg", "publish", []string{
				"-re",
				"-stream_loop", "-1",
				"-i", "emptyvideo.ts",
				"-c", "copy",
				"-f", "rtsp",
				"-rtsp_transport", "udp",
				"rtsp://localhost:8554/teststream",
			})
			require.NoError(t, err)
			defer cnt2.close()

			time.Sleep(1 * time.Second)

			conf := ClientConf{
				StreamProtocol: func() *StreamProtocol {
					if proto == "udp" {
						v := StreamProtocolUDP
						return &v
					}
					v := StreamProtocolTCP
					return &v
				}(),
			}

			conn, err := conf.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			firstFrame := int32(0)
			frameRecv := make(chan struct{})
			done := conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
				if atomic.SwapInt32(&firstFrame, 1) == 0 {
					close(frameRecv)
				}
			})

			<-frameRecv
			_, err = conn.Pause()
			require.NoError(t, err)
			<-done

			_, err = conn.Play()
			require.NoError(t, err)

			firstFrame = int32(0)
			frameRecv = make(chan struct{})
			done = conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
				if atomic.SwapInt32(&firstFrame, 1) == 0 {
					close(frameRecv)
				}
			})

			<-frameRecv
			conn.Close()
			<-done
		})
	}
}
