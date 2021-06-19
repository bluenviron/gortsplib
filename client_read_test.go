package gortsplib

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/pkg/auth"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
)

func TestClientReadTracks(t *testing.T) {
	track1, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	track2, err := NewTrackAAC(96, []byte{17, 144})
	require.NoError(t, err)

	track3, err := NewTrackAAC(96, []byte{0x12, 0x30})
	require.NoError(t, err)

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

		req, err := readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: Tracks{track1, track2, track3}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			req, err := readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL(fmt.Sprintf("rtsp://localhost:8554/teststream/trackID=%d", i)), req.URL)

			var inTH headers.Transport
			err = inTH.Read(req.Header["Transport"])
			require.NoError(t, err)

			th := headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Protocol:    base.StreamProtocolUDP,
				ClientPorts: inTH.ClientPorts,
				ServerPorts: &[2]int{34556 + i*2, 34557 + i*2},
			}

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)
		}

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	conn, err := DialRead("rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	defer conn.Close()

	track1.Media.Attributes = append(track1.Media.Attributes, psdp.Attribute{
		Key:   "control",
		Value: "trackID=0",
	})

	track2.Media.Attributes = append(track2.Media.Attributes, psdp.Attribute{
		Key:   "control",
		Value: "trackID=1",
	})

	track3.Media.Attributes = append(track3.Media.Attributes, psdp.Attribute{
		Key:   "control",
		Value: "trackID=2",
	})

	require.Equal(t, Tracks{
		{
			ID:      0,
			BaseURL: mustParseURL("rtsp://localhost:8554/teststream/"),
			Media:   track1.Media,
		},
		{
			ID:      1,
			BaseURL: mustParseURL("rtsp://localhost:8554/teststream/"),
			Media:   track2.Media,
		},
		{
			ID:      2,
			BaseURL: mustParseURL("rtsp://localhost:8554/teststream/"),
			Media:   track3.Media,
		},
	}, conn.Tracks())
}

func TestClientRead(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"multicast",
		"tcp",
		"tls",
	} {
		t.Run(proto, func(t *testing.T) {
			frameRecv := make(chan struct{})

			listenIP := multicastCapableIP(t)
			l, err := net.Listen("tcp", listenIP+":8554")
			require.NoError(t, err)
			defer l.Close()

			var scheme string
			if proto == "tls" {
				scheme = "rtsps"

				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)

				l = tls.NewListener(l, &tls.Config{Certificates: []tls.Certificate{cert}})

			} else {
				scheme = "rtsp"
			}

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				req, err := readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/teststream"), req.URL)

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

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/teststream"), req.URL)

				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{scheme + "://" + listenIP + ":8554/teststream/"},
					},
					Body: Tracks{track}.Write(),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/teststream/trackID=0"), req.URL)

				var inTH headers.Transport
				err = inTH.Read(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{}

				var l1 net.PacketConn
				var l2 net.PacketConn

				switch proto {
				case "udp":
					v := base.StreamDeliveryUnicast
					th.Delivery = &v
					th.Protocol = base.StreamProtocolUDP
					th.ClientPorts = inTH.ClientPorts
					th.ServerPorts = &[2]int{34556, 34557}

					l1, err = net.ListenPacket("udp", listenIP+":34556")
					require.NoError(t, err)
					defer l1.Close()

					l2, err = net.ListenPacket("udp", listenIP+":34557")
					require.NoError(t, err)
					defer l2.Close()

				case "multicast":
					v := base.StreamDeliveryMulticast
					th.Delivery = &v
					th.Protocol = base.StreamProtocolUDP
					v2 := net.ParseIP("224.1.0.1")
					th.Destination = &v2
					th.Ports = &[2]int{25000, 25001}

					l1, err = net.ListenPacket("udp", "224.0.0.0:25000")
					require.NoError(t, err)
					defer l1.Close()

					p := ipv4.NewPacketConn(l1)

					intfs, err := net.Interfaces()
					require.NoError(t, err)

					for _, intf := range intfs {
						err := p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP("224.1.0.1")})
						require.NoError(t, err)
					}

					l2, err = net.ListenPacket("udp", "224.0.0.0:25001")
					require.NoError(t, err)
					defer l2.Close()

					p = ipv4.NewPacketConn(l2)

					intfs, err = net.Interfaces()
					require.NoError(t, err)

					for _, intf := range intfs {
						err := p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP("224.1.0.1")})
						require.NoError(t, err)
					}

				case "tcp", "tls":
					v := base.StreamDeliveryUnicast
					th.Delivery = &v
					th.Protocol = base.StreamProtocolTCP
					th.InterleavedIDs = &[2]int{0, 1}
				}

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/teststream/"), req.URL)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				// server -> client
				switch proto {
				case "udp":
					time.Sleep(1 * time.Second)
					l1.WriteTo([]byte{0x01, 0x02, 0x03, 0x04}, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})

				case "multicast":
					time.Sleep(1 * time.Second)
					l1.WriteTo([]byte{0x01, 0x02, 0x03, 0x04}, &net.UDPAddr{
						IP:   net.ParseIP("224.1.0.1"),
						Port: 25000,
					})

				case "tcp", "tls":
					err = base.InterleavedFrame{
						TrackID:    0,
						StreamType: StreamTypeRTP,
						Payload:    []byte{0x01, 0x02, 0x03, 0x04},
					}.Write(bconn.Writer)
					require.NoError(t, err)
				}

				// client -> server (RTCP)
				switch proto {
				case "udp":
					// skip firewall opening
					buf := make([]byte, 2048)
					_, _, err := l2.ReadFrom(buf)
					require.NoError(t, err)

					buf = make([]byte, 2048)
					n, _, err := l2.ReadFrom(buf)
					require.NoError(t, err)
					require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, buf[:n])
					close(frameRecv)

				case "tcp", "tls":
					var f base.InterleavedFrame
					f.Payload = make([]byte, 2048)
					err := f.Read(bconn.Reader)
					require.NoError(t, err)
					require.Equal(t, 0, f.TrackID)
					require.Equal(t, StreamTypeRTCP, f.StreamType)
					require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, f.Payload)
					close(frameRecv)
				}

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/teststream/"), req.URL)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}()

			c := &Client{
				Protocol: func() *ClientProtocol {
					switch proto {
					case "udp":
						v := ClientProtocolUDP
						return &v

					case "multicast":
						v := ClientProtocolMulticast
						return &v

					default: // tcp, tls
						v := ClientProtocolTCP
						return &v
					}
				}(),
			}

			conn, err := c.DialRead(scheme + "://" + listenIP + ":8554/teststream")
			require.NoError(t, err)

			done := make(chan struct{})
			counter := uint64(0)
			go func() {
				defer close(done)
				conn.ReadFrames(func(id int, streamType StreamType, payload []byte) {
					// skip multicast loopback
					if proto == "multicast" && atomic.AddUint64(&counter, 1) <= 2 {
						return
					}

					require.Equal(t, 0, id)
					require.Equal(t, StreamTypeRTP, streamType)
					require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, payload)

					if proto != "multicast" {
						err = conn.WriteFrame(0, StreamTypeRTCP, []byte{0x05, 0x06, 0x07, 0x08})
						require.NoError(t, err)
					} else {
						close(frameRecv)
					}
				})
			}()

			<-frameRecv
			conn.Close()
			<-done

			conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
			})
		})
	}
}

func TestClientReadNoContentBase(t *testing.T) {
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

		req, err := readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

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

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

		var inTH headers.Transport
		err = inTH.Read(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery: func() *base.StreamDelivery {
				v := base.StreamDeliveryUnicast
				return &v
			}(),
			Protocol:    base.StreamProtocolUDP,
			ClientPorts: inTH.ClientPorts,
			ServerPorts: &[2]int{34556, 34557},
		}

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Write(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	conn, err := DialRead("rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	conn.Close()
}

func TestClientReadAnyPort(t *testing.T) {
	for _, ca := range []string{
		"zero",
		"zero_one",
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

				req, err := readRequest(bconn.Reader)
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

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: Tracks{track}.Write(),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var th headers.Transport
				err = th.Read(req.Header["Transport"])
				require.NoError(t, err)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": headers.Transport{
							Protocol: base.StreamProtocolUDP,
							Delivery: func() *base.StreamDelivery {
								v := base.StreamDeliveryUnicast
								return &v
							}(),
							ClientPorts: th.ClientPorts,
							ServerPorts: func() *[2]int {
								switch ca {
								case "zero":
									return &[2]int{0, 0}

								case "zero_one":
									return &[2]int{0, 1}
								}
								return nil
							}(),
						}.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
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

			c := &Client{
				AnyPortEnable: true,
			}

			conn, err := c.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			frameRecv := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
					close(frameRecv)
				})
			}()

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

			req, err := readRequest(bconn.Reader)
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

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
					"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
				},
				Body: Tracks{track}.Write(),
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			err = base.Response{
				StatusCode: base.StatusUnsupportedTransport,
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			var inTH headers.Transport
			err = inTH.Read(req.Header["Transport"])
			require.NoError(t, err)
			require.Equal(t, base.StreamProtocolTCP, inTH.Protocol)

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": headers.Transport{
						Protocol: base.StreamProtocolTCP,
						Delivery: func() *base.StreamDelivery {
							v := base.StreamDeliveryUnicast
							return &v
						}(),
						InterleavedIDs: &[2]int{0, 1},
					}.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
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
		done := make(chan struct{})
		go func() {
			defer close(done)
			conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
				close(frameRecv)
			})
		}()

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

			req, err := readRequest(bconn.Reader)
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

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			v := auth.NewValidator("myuser", "mypass", nil)

			err = base.Response{
				StatusCode: base.StatusUnauthorized,
				Header: base.Header{
					"WWW-Authenticate": v.Header(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			err = v.ValidateRequest(req, nil)
			require.NoError(t, err)

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
					"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
				},
				Body: Tracks{track}.Write(),
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

			var inTH headers.Transport
			err = inTH.Read(req.Header["Transport"])
			require.NoError(t, err)

			th := headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Protocol:    base.StreamProtocolUDP,
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

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Play, req.Method)

			err = base.Response{
				StatusCode: base.StatusOK,
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
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

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			v = auth.NewValidator("myuser", "mypass", nil)

			err = base.Response{
				StatusCode: base.StatusUnauthorized,
				Header: base.Header{
					"WWW-Authenticate": v.Header(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

			err = v.ValidateRequest(req, nil)
			require.NoError(t, err)

			inTH = headers.Transport{}
			err = inTH.Read(req.Header["Transport"])
			require.NoError(t, err)

			th = headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Protocol:       base.StreamProtocolTCP,
				InterleavedIDs: inTH.InterleavedIDs,
			}

			err = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			req, err = readRequest(bconn.Reader)
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

			req, err = readRequest(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.Teardown, req.Method)

			err = base.Response{
				StatusCode: base.StatusOK,
			}.Write(bconn.Writer)
			require.NoError(t, err)

			conn.Close()
		}()

		c := &Client{
			ReadTimeout: 1 * time.Second,
		}

		conn, err := c.DialRead("rtsp://myuser:mypass@localhost:8554/teststream")
		require.NoError(t, err)

		frameRecv := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
				close(frameRecv)
			})
		}()

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

		req, err := readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var th headers.Transport
		err = th.Read(req.Header["Transport"])
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol: base.StreamProtocolUDP,
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

		req, err = readRequest(bconn.Reader)
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
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
			close(frameRecv)
		})
	}()

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
			if inTH.Protocol == base.StreamProtocolUDP {
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
					if inTH.Protocol == base.StreamProtocolUDP {
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
				defer conn.Close()
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				req, err := readRequest(bconn.Reader)
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

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: Tracks{track}.Write(),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
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
					th.Protocol = base.StreamProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts

				} else {
					th.Protocol = base.StreamProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				writerTerminate, writerDone := writeFrames(&inTH, bconn)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				close(writerTerminate)
				<-writerDone

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				writerTerminate, writerDone = writeFrames(&inTH, bconn)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				close(writerTerminate)
				<-writerDone

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}()

			c := &Client{
				Protocol: func() *ClientProtocol {
					if proto == "udp" {
						v := ClientProtocolUDP
						return &v
					}
					v := ClientProtocolTCP
					return &v
				}(),
			}

			conn, err := c.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			firstFrame := int32(0)
			frameRecv := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
					if atomic.SwapInt32(&firstFrame, 1) == 0 {
						close(frameRecv)
					}
				})
			}()

			<-frameRecv
			_, err = conn.Pause()
			require.NoError(t, err)
			<-done

			conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
			})

			_, err = conn.Play(nil)
			require.NoError(t, err)

			firstFrame = int32(0)
			frameRecv = make(chan struct{})
			done = make(chan struct{})
			go func() {
				defer close(done)
				conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
					if atomic.SwapInt32(&firstFrame, 1) == 0 {
						close(frameRecv)
					}
				})
			}()

			<-frameRecv
			conn.Close()
			<-done
		})
	}
}

func TestClientReadRTCPReport(t *testing.T) {
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

		req, err := readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var th headers.Transport
		err = th.Read(req.Header["Transport"])
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol: base.StreamProtocolTCP,
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

		req, err = readRequest(bconn.Reader)
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

	c := &Client{
		Protocol: func() *ClientProtocol {
			v := ClientProtocolTCP
			return &v
		}(),
		receiverReportPeriod: 1 * time.Second,
	}

	conn, err := c.DialRead("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	recv := 0
	recvDone := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.ReadFrames(func(id int, typ StreamType, payload []byte) {
			recv++
			if recv >= 3 {
				close(recvDone)
			}
		})
	}()

	time.Sleep(1300 * time.Millisecond)

	<-recvDone
	conn.Close()
	<-done
}

func TestClientReadErrorTimeout(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
		"auto",
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
				defer conn.Close()
				bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

				req, err := readRequest(bconn.Reader)
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

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: Tracks{track}.Write(),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
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

				var l1 net.PacketConn
				if proto == "udp" || proto == "auto" {
					var err error
					l1, err = net.ListenPacket("udp", "localhost:34557")
					require.NoError(t, err)
					defer l1.Close()

					th.Protocol = base.StreamProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts

				} else {
					th.Protocol = base.StreamProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Write(),
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				if proto == "udp" || proto == "auto" {
					time.Sleep(500 * time.Millisecond)

					l1, err := net.ListenPacket("udp", "localhost:34556")
					require.NoError(t, err)
					defer l1.Close()

					// write a packet to skip the protocol autodetection feature
					l1.WriteTo([]byte("\x01\x02\x03\x04"), &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})
				}

				req, err = readRequest(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}()

			c := &Client{
				Protocol: func() *ClientProtocol {
					switch proto {
					case "udp":
						v := ClientProtocolUDP
						return &v

					case "tcp":
						v := ClientProtocolTCP
						return &v
					}
					return nil
				}(),
				InitialUDPReadTimeout: 1 * time.Second,
				ReadTimeout:           1 * time.Second,
			}

			conn, err := c.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)
			defer conn.Close()

			err = conn.ReadFrames(func(trackID int, streamType StreamType, payload []byte) {
			})

			switch proto {
			case "udp", "auto":
				require.Equal(t, "UDP timeout", err.Error())

			case "tcp":
				require.Equal(t, "TCP timeout", err.Error())
			}
		})
	}
}

func TestClientReadIgnoreTCPInvalidTrack(t *testing.T) {
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

		req, err := readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
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
		th.Protocol = base.StreamProtocolTCP
		th.InterleavedIDs = inTH.InterleavedIDs

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Write(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = base.InterleavedFrame{
			TrackID:    3,
			StreamType: StreamTypeRTP,
			Payload:    []byte{0x01, 0x02, 0x03, 0x04},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = base.InterleavedFrame{
			TrackID:    0,
			StreamType: StreamTypeRTP,
			Payload:    []byte{0x05, 0x06, 0x07, 0x08},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	c := &Client{
		Protocol: func() *ClientProtocol {
			v := ClientProtocolTCP
			return &v
		}(),
	}

	conn, err := c.DialRead("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	recv := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.ReadFrames(func(trackID int, streamType StreamType, payload []byte) {
			close(recv)
		})
	}()

	<-recv
	conn.Close()
	<-done
}

func TestClientReadSeek(t *testing.T) {
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

		req, err := readRequest(bconn.Reader)
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

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
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
			Protocol:       base.StreamProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Write(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		var ra headers.Range
		err = ra.Read(req.Header["Range"])
		require.NoError(t, err)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: headers.RangeNPTTime(5500 * time.Millisecond),
			},
		}, ra)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Pause, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = ra.Read(req.Header["Range"])
		require.NoError(t, err)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: headers.RangeNPTTime(6400 * time.Millisecond),
			},
		}, ra)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	c := &Client{
		Protocol: func() *ClientProtocol {
			v := ClientProtocolTCP
			return &v
		}(),
	}

	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	conn, err := c.Dial(u.Scheme, u.Host)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	tracks, _, err := conn.Describe(u)
	require.NoError(t, err)

	for _, track := range tracks {
		_, err := conn.Setup(headers.TransportModePlay, track, 0, 0)
		require.NoError(t, err)
	}

	_, err = conn.Play(&headers.Range{
		Value: &headers.RangeNPT{
			Start: headers.RangeNPTTime(5500 * time.Millisecond),
		},
	})
	require.NoError(t, err)

	_, err = conn.Seek(&headers.Range{
		Value: &headers.RangeNPT{
			Start: headers.RangeNPTTime(6400 * time.Millisecond),
		},
	})
	require.NoError(t, err)
}
