package gortsplib

import (
	"bufio"
	"crypto/tls"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
)

func TestClientRead(t *testing.T) {
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

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
					},
					Body: Tracks{track}.Write(),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var th headers.Transport
				err = th.Read(req.Header["Transport"])
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
								InterleavedIDs: &[2]int{0, 1},
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

func TestClientReadAnyPort(t *testing.T) {
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

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
					},
					Body: Tracks{track}.Write(),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var th headers.Transport
				err = th.Read(req.Header["Transport"])
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

func TestClientReadAutomaticProtocol(t *testing.T) {
	t.Run("switch after status code", func(t *testing.T) {
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

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: Tracks{track}.Write(),
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
						InterleavedIDs: &[2]int{0, 1},
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

		conn, err := DialRead("rtsp://localhost:8554/teststream")
		require.NoError(t, err)

		frameRecv := make(chan struct{})
		done := conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
			close(frameRecv)
		})

		<-frameRecv
		conn.Close()
		<-done
	})

	t.Run("switch after timeout", func(t *testing.T) {
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

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: Tracks{track}.Write(),
			}.Write(bconn.Writer)
			require.NoError(t, err)

			err = req.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			var inTH headers.Transport
			err = inTH.Read(req.Header["Transport"])
			require.NoError(t, err)

			th := headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Protocol:    StreamProtocolUDP,
				ServerPorts: &[2]int{34556, 34557},
				ClientPorts: inTH.ClientPorts,
			}

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Write(),
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

			err = req.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Teardown, req.Method)

			err = base.Response{
				StatusCode: base.StatusOK,
			}.Write(bconn.Writer)
			require.NoError(t, err)

			conn.Close()

			conn, err = l.Accept()
			require.NoError(t, err)
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
			require.Equal(t, base.Setup, req.Method)

			inTH = headers.Transport{}
			err = inTH.Read(req.Header["Transport"])
			require.NoError(t, err)

			th = headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Protocol:       StreamProtocolTCP,
				InterleavedIDs: inTH.InterleavedIDs,
			}

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Write(),
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

			base.InterleavedFrame{
				TrackID:    0,
				StreamType: StreamTypeRTP,
				Payload:    []byte("\x00\x00\x00\x00"),
			}.Write(bconn.Writer)

			err = req.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Teardown, req.Method)

			err = base.Response{
				StatusCode: base.StatusOK,
			}.Write(bconn.Writer)
			require.NoError(t, err)

			conn.Close()
		}()

		conf := ClientConf{
			ReadTimeout: 1 * time.Second,
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

func TestClientReadRedirect(t *testing.T) {
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

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var th headers.Transport
		err = th.Read(req.Header["Transport"])
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

func TestClientReadPause(t *testing.T) {
	writeFrames := func(inTH *headers.Transport, bconn *bufio.ReadWriter) (chan struct{}, chan struct{}) {
		writerTerminate := make(chan struct{})
		writerDone := make(chan struct{})

		go func() {
			defer close(writerDone)

			var l1 net.PacketConn
			if inTH.Protocol == StreamProtocolUDP {
				var err error
				l1, err = net.ListenPacket("udp", "localhost:34556")
				require.NoError(t, err)
				defer l1.Close()
			}

			t := time.NewTicker(50 * time.Millisecond)
			defer t.Stop()

			for {
				select {
				case <-t.C:
					if inTH.Protocol == StreamProtocolUDP {
						l1.WriteTo([]byte("\x00\x00\x00\x00"), &net.UDPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: inTH.ClientPorts[0],
						})
					} else {
						base.InterleavedFrame{
							TrackID:    0,
							StreamType: StreamTypeRTP,
							Payload:    []byte("\x00\x00\x00\x00"),
						}.Write(bconn.Writer)
					}

				case <-writerTerminate:
					return
				}
			}
		}()

		return writerTerminate, writerDone
	}

	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
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

				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
					},
					Body: Tracks{track}.Write(),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Read(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *base.StreamDelivery {
						v := base.StreamDeliveryUnicast
						return &v
					}(),
				}

				if proto == "udp" {
					th.Protocol = StreamProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts

				} else {
					th.Protocol = StreamProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
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

				writerTerminate, writerDone := writeFrames(&inTH, bconn)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				close(writerTerminate)
				<-writerDone

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				writerTerminate, writerDone = writeFrames(&inTH, bconn)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				close(writerTerminate)
				<-writerDone

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				conn.Close()
			}()

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

func TestClientReadRTCP(t *testing.T) {
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

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var th headers.Transport
		err = th.Read(req.Header["Transport"])
		require.NoError(t, err)

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
					InterleavedIDs: &[2]int{0, 1},
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

		rs := rtcpsender.New(90000)

		byts, _ := (&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 946,
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
		rs.ProcessFrame(time.Now(), StreamTypeRTP, byts)

		err = base.InterleavedFrame{
			TrackID:    0,
			StreamType: StreamTypeRTCP,
			Payload:    rs.Report(time.Now()),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		var f base.InterleavedFrame
		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
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
					LastSequenceNumber: 946,
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
	}()

	conf := ClientConf{
		StreamProtocol: func() *StreamProtocol {
			v := StreamProtocolTCP
			return &v
		}(),
		receiverReportPeriod: 1 * time.Second,
	}

	conn, err := conf.DialRead("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	recv := 0
	recvDone := make(chan struct{})
	done := conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
		recv++
		if recv >= 3 {
			close(recvDone)
		}
	})

	time.Sleep(1300 * time.Millisecond)

	<-recvDone
	conn.Close()
	<-done
}

func TestClientReadWriteManualRTCP(t *testing.T) {
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

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var th headers.Transport
		err = th.Read(req.Header["Transport"])
		require.NoError(t, err)

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
					InterleavedIDs: &[2]int{0, 1},
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

		var f base.InterleavedFrame
		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, StreamTypeRTCP, f.StreamType)
		require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, f.Payload)
	}()

	conf := ClientConf{
		StreamProtocol: func() *StreamProtocol {
			v := StreamProtocolTCP
			return &v
		}(),
	}

	conn, err := conf.DialRead("rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	defer conn.Close()

	err = conn.WriteFrame(0, StreamTypeRTCP, []byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
}
