package gortsplib

import (
	"bytes"
	"crypto/tls"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/v2/pkg/auth"
	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/conn"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/headers"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/aler9/gortsplib/v2/pkg/mpeg4audio"
	"github.com/aler9/gortsplib/v2/pkg/sdp"
	"github.com/aler9/gortsplib/v2/pkg/url"
)

func mustMarshalSDP(sdp *sdp.SessionDescription) []byte {
	byts, err := sdp.Marshal()
	if err != nil {
		panic(err)
	}
	return byts
}

func readAll(c *Client, ur string, cb func(*media.Media, format.Format, *rtp.Packet)) error {
	u, err := url.Parse(ur)
	if err != nil {
		return err
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	medias, baseURL, _, err := c.Describe(u)
	if err != nil {
		c.Close()
		return err
	}

	err = c.SetupAll(medias, baseURL)
	if err != nil {
		c.Close()
		return err
	}

	if cb != nil {
		c.OnPacketRTPAny(cb)
	}

	_, err = c.Play(nil)
	if err != nil {
		c.Close()
		return err
	}

	return nil
}

func TestClientPlayFormats(t *testing.T) {
	media1 := testH264Media

	media2 := &media.Media{
		Type: media.TypeAudio,
		Formats: []format.Format{&format.MPEG4Audio{
			PayloadTyp: 96,
			Config: &mpeg4audio.Config{
				Type:         mpeg4audio.ObjectTypeAACLC,
				SampleRate:   44100,
				ChannelCount: 2,
			},
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		}},
	}

	media3 := &media.Media{
		Type: media.TypeAudio,
		Formats: []format.Format{&format.MPEG4Audio{
			PayloadTyp: 96,
			Config: &mpeg4audio.Config{
				Type:         mpeg4audio.ObjectTypeAACLC,
				SampleRate:   96000,
				ChannelCount: 2,
			},
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		}},
	}

	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		medias := media.Medias{media1, media2, media3}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			req, err := conn.ReadRequest()
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[i].Control), req.URL)

			var inTH headers.Transport
			err = inTH.Unmarshal(req.Header["Transport"])
			require.NoError(t, err)

			th := headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Protocol:    headers.TransportProtocolUDP,
				ClientPorts: inTH.ClientPorts,
				ServerPorts: &[2]int{34556 + i*2, 34557 + i*2},
			}

			err = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Marshal(),
				},
			})
			require.NoError(t, err)
		}

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	c := Client{}

	err = readAll(&c, "rtsp://localhost:8554/teststream", nil)
	require.NoError(t, err)
	defer c.Close()
}

func TestClientPlay(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"multicast",
		"tcp",
		"tls",
	} {
		t.Run(transport, func(t *testing.T) {
			packetRecv := make(chan struct{})

			listenIP := multicastCapableIP(t)
			l, err := net.Listen("tcp", listenIP+":8554")
			require.NoError(t, err)
			defer l.Close()

			var scheme string
			if transport == "tls" {
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

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value"), req.URL)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value"), req.URL)

				forma := &format.Generic{
					PayloadTyp: 96,
					RTPMap:     "private/90000",
				}
				err = forma.Init()
				require.NoError(t, err)

				medias := media.Medias{
					&media.Media{
						Type:    "application",
						Formats: []format.Format{forma},
					},
					&media.Media{
						Type:    "application",
						Formats: []format.Format{forma},
					},
				}
				medias.SetControls()

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{scheme + "://" + listenIP + ":8554/test/stream?param=value/"},
					},
					Body: mustMarshalSDP(medias.Marshal(false)),
				})
				require.NoError(t, err)

				var l1s [2]net.PacketConn
				var l2s [2]net.PacketConn
				var clientPorts [2]*[2]int

				for i := 0; i < 2; i++ {
					req, err = conn.ReadRequest()
					require.NoError(t, err)
					require.Equal(t, base.Setup, req.Method)
					require.Equal(t, mustParseURL(
						scheme+"://"+listenIP+":8554/test/stream?param=value/"+medias[i].Control), req.URL)

					var inTH headers.Transport
					err = inTH.Unmarshal(req.Header["Transport"])
					require.NoError(t, err)

					var th headers.Transport

					switch transport {
					case "udp":
						v := headers.TransportDeliveryUnicast
						th.Delivery = &v
						th.Protocol = headers.TransportProtocolUDP
						th.ClientPorts = inTH.ClientPorts
						clientPorts[i] = inTH.ClientPorts
						th.ServerPorts = &[2]int{34556 + i*2, 34557 + i*2}

						l1s[i], err = net.ListenPacket("udp", listenIP+":"+strconv.FormatInt(int64(th.ServerPorts[0]), 10))
						require.NoError(t, err)
						defer l1s[i].Close()

						l2s[i], err = net.ListenPacket("udp", listenIP+":"+strconv.FormatInt(int64(th.ServerPorts[1]), 10))
						require.NoError(t, err)
						defer l2s[i].Close()

					case "multicast":
						v := headers.TransportDeliveryMulticast
						th.Delivery = &v
						th.Protocol = headers.TransportProtocolUDP
						v2 := net.ParseIP("224.1.0.1")
						th.Destination = &v2
						th.Ports = &[2]int{25000 + i*2, 25001 + i*2}

						l1s[i], err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[0]), 10))
						require.NoError(t, err)
						defer l1s[i].Close()

						p := ipv4.NewPacketConn(l1s[i])

						intfs, err := net.Interfaces()
						require.NoError(t, err)

						for _, intf := range intfs {
							err := p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP("224.1.0.1")})
							require.NoError(t, err)
						}

						l2s[i], err = net.ListenPacket("udp", "224.0.0.0:25001")
						require.NoError(t, err)
						defer l2s[i].Close()

						p = ipv4.NewPacketConn(l2s[i])

						intfs, err = net.Interfaces()
						require.NoError(t, err)

						for _, intf := range intfs {
							err := p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP("224.1.0.1")})
							require.NoError(t, err)
						}

					case "tcp", "tls":
						v := headers.TransportDeliveryUnicast
						th.Delivery = &v
						th.Protocol = headers.TransportProtocolTCP
						th.InterleavedIDs = &[2]int{0 + i*2, 1 + i*2}
					}

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Transport": th.Marshal(),
						},
					})
					require.NoError(t, err)
				}

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value/"), req.URL)
				require.Equal(t, base.HeaderValue{"npt=0-"}, req.Header["Range"])

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				for i := 0; i < 2; i++ {
					// server -> client (RTP)
					switch transport {
					case "udp":
						l1s[i].WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: clientPorts[i][0],
						})

					case "multicast":
						l1s[i].WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
							IP:   net.ParseIP("224.1.0.1"),
							Port: 25000,
						})

					case "tcp", "tls":
						err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
							Channel: 0 + i*2,
							Payload: testRTPPacketMarshaled,
						}, make([]byte, 1024))
						require.NoError(t, err)
					}

					// client -> server (RTCP)
					switch transport {
					case "udp", "multicast":
						// skip firewall opening
						if transport == "udp" {
							buf := make([]byte, 2048)
							_, _, err := l2s[i].ReadFrom(buf)
							require.NoError(t, err)
						}

						buf := make([]byte, 2048)
						n, _, err := l2s[i].ReadFrom(buf)
						require.NoError(t, err)
						packets, err := rtcp.Unmarshal(buf[:n])
						require.NoError(t, err)
						require.Equal(t, &testRTCPPacket, packets[0])

					case "tcp", "tls":
						f, err := conn.ReadInterleavedFrame()
						require.NoError(t, err)
						require.Equal(t, 1+i*2, f.Channel)
						packets, err := rtcp.Unmarshal(f.Payload)
						require.NoError(t, err)
						require.Equal(t, &testRTCPPacket, packets[0])
					}
				}

				close(packetRecv)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value/"), req.URL)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			c := Client{
				TLSConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				Transport: func() *Transport {
					switch transport {
					case "udp":
						v := TransportUDP
						return &v

					case "multicast":
						v := TransportUDPMulticast
						return &v

					default: // tcp, tls
						v := TransportTCP
						return &v
					}
				}(),
			}

			u, err := url.Parse(scheme + "://" + listenIP + ":8554/test/stream?param=value")
			require.NoError(t, err)

			err = c.Start(u.Scheme, u.Host)
			require.NoError(t, err)
			defer c.Close()

			medias, baseURL, _, err := c.Describe(u)
			require.NoError(t, err)

			err = c.SetupAll(medias, baseURL)
			require.NoError(t, err)

			c.OnPacketRTPAny(func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
				require.Equal(t, &testRTPPacket, pkt)
				err := c.WritePacketRTCP(medi, &testRTCPPacket)
				require.NoError(t, err)
			})

			_, err = c.Play(nil)
			require.NoError(t, err)

			<-packetRecv
		})
	}
}

func TestClientPlayPartial(t *testing.T) {
	listenIP := multicastCapableIP(t)
	l, err := net.Listen("tcp", listenIP+":8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream"), req.URL)

		forma := &format.Generic{
			PayloadTyp: 96,
			RTPMap:     "private/90000",
		}
		err = forma.Init()
		require.NoError(t, err)

		medias := media.Medias{
			&media.Media{
				Type:    "application",
				Formats: []format.Format{forma},
			},
			&media.Media{
				Type:    "application",
				Formats: []format.Format{forma},
			},
		}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://" + listenIP + ":8554/teststream/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"+medias[1].Control), req.URL)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)
		require.Equal(t, &[2]int{0, 1}, inTH.InterleavedIDs)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	packetRecv := make(chan struct{})

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
	}

	u, err := url.Parse("rtsp://" + listenIP + ":8554/teststream")
	require.NoError(t, err)

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	medias, baseURL, _, err := c.Describe(u)
	require.NoError(t, err)

	_, err = c.Setup(medias[1], baseURL, 0, 0)
	require.NoError(t, err)

	c.OnPacketRTPAny(func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
		require.Equal(t, medias[1], medi)
		require.Equal(t, medias[1].Formats[0], forma)
		require.Equal(t, &testRTPPacket, pkt)
		close(packetRecv)
	})

	_, err = c.Play(nil)
	require.NoError(t, err)

	<-packetRecv
}

func TestClientPlayContentBase(t *testing.T) {
	for _, ca := range []string{
		"absent",
		"inside control attribute",
	} {
		t.Run(ca, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				medias := media.Medias{testH264Media}
				medias.SetControls()

				switch ca {
				case "absent":
					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
						},
						Body: mustMarshalSDP(medias.Marshal(false)),
					})
					require.NoError(t, err)

				case "inside control attribute":
					body := string(mustMarshalSDP(medias.Marshal(false)))
					body = strings.Replace(body, "t=0 0", "t=0 0\r\na=control:rtsp://localhost:8554/teststream", 1)

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
							"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream2/"},
						},
						Body: []byte(body),
					})
					require.NoError(t, err)
				}

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
					Protocol:    headers.TransportProtocolUDP,
					ClientPorts: inTH.ClientPorts,
					ServerPorts: &[2]int{34556, 34557},
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			c := Client{}

			err = readAll(&c, "rtsp://localhost:8554/teststream", nil)
			require.NoError(t, err)
			c.Close()
		})
	}
}

func TestClientPlayAnyPort(t *testing.T) {
	for _, ca := range []string{
		"zero",
		"zero_one",
		"no",
		"random",
	} {
		t.Run(ca, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverRecv := make(chan struct{})

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				medias := media.Medias{testH264Media}
				medias.SetControls()

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mustMarshalSDP(medias.Marshal(false)),
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var th headers.Transport
				err = th.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				l1a, err := net.ListenPacket("udp", "localhost:13344")
				require.NoError(t, err)
				defer l1a.Close()

				l1b, err := net.ListenPacket("udp", "localhost:23041")
				require.NoError(t, err)
				defer l1b.Close()

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": headers.Transport{
							Protocol: headers.TransportProtocolUDP,
							Delivery: func() *headers.TransportDelivery {
								v := headers.TransportDeliveryUnicast
								return &v
							}(),
							ClientPorts: th.ClientPorts,
							ServerPorts: func() *[2]int {
								switch ca {
								case "zero":
									return &[2]int{0, 0}

								case "zero_one":
									return &[2]int{0, 1}

								case "no":
									return nil

								default: // random
									return &[2]int{23040, 23041}
								}
							}(),
						}.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				time.Sleep(500 * time.Millisecond)

				l1a.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ClientPorts[0],
				})

				// read RTCP
				if ca == "random" {
					// skip firewall opening
					buf := make([]byte, 2048)
					_, _, err := l1b.ReadFrom(buf)
					require.NoError(t, err)

					buf = make([]byte, 2048)
					n, _, err := l1b.ReadFrom(buf)
					require.NoError(t, err)
					packets, err := rtcp.Unmarshal(buf[:n])
					require.NoError(t, err)
					require.Equal(t, &testRTCPPacket, packets[0])
					close(serverRecv)
				}
			}()

			packetRecv := make(chan struct{})

			c := Client{
				AnyPortEnable: true,
			}

			var med *media.Media
			err = readAll(&c, "rtsp://localhost:8554/teststream",
				func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
					require.Equal(t, &testRTPPacket, pkt)
					med = medi
					close(packetRecv)
				})
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv

			if ca == "random" {
				c.WritePacketRTCP(med, &testRTCPPacket)
				<-serverRecv
			}
		})
	}
}

func TestClientPlayAutomaticProtocol(t *testing.T) {
	t.Run("switch after status code", func(t *testing.T) {
		l, err := net.Listen("tcp", "localhost:8554")
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()
		go func() {
			defer close(serverDone)

			nconn, err := l.Accept()
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			req, err := conn.ReadRequest()
			require.NoError(t, err)
			require.Equal(t, base.Options, req.Method)

			err = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Public": base.HeaderValue{strings.Join([]string{
						string(base.Describe),
						string(base.Setup),
						string(base.Play),
					}, ", ")},
				},
			})
			require.NoError(t, err)

			req, err = conn.ReadRequest()
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			medias := media.Medias{testH264Media}
			medias.SetControls()

			err = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
					"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
				},
				Body: mustMarshalSDP(medias.Marshal(false)),
			})
			require.NoError(t, err)

			req, err = conn.ReadRequest()
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			err = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusUnsupportedTransport,
			})
			require.NoError(t, err)

			req, err = conn.ReadRequest()
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			var inTH headers.Transport
			err = inTH.Unmarshal(req.Header["Transport"])
			require.NoError(t, err)
			require.Equal(t, headers.TransportProtocolTCP, inTH.Protocol)

			err = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": headers.Transport{
						Protocol: headers.TransportProtocolTCP,
						Delivery: func() *headers.TransportDelivery {
							v := headers.TransportDeliveryUnicast
							return &v
						}(),
						InterleavedIDs: &[2]int{0, 1},
					}.Marshal(),
				},
			})
			require.NoError(t, err)

			req, err = conn.ReadRequest()
			require.NoError(t, err)
			require.Equal(t, base.Play, req.Method)

			err = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
			})
			require.NoError(t, err)

			err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
				Channel: 0,
				Payload: testRTPPacketMarshaled,
			}, make([]byte, 1024))
			require.NoError(t, err)
		}()

		packetRecv := make(chan struct{})

		c := Client{}
		err = readAll(&c, "rtsp://localhost:8554/teststream",
			func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
				close(packetRecv)
			})
		require.NoError(t, err)
		defer c.Close()

		<-packetRecv
	})

	t.Run("switch after timeout", func(t *testing.T) {
		l, err := net.Listen("tcp", "localhost:8554")
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()
		go func() {
			defer close(serverDone)

			medias := media.Medias{testH264Media}
			medias.SetControls()

			func() {
				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				v := auth.NewValidator("myuser", "mypass", nil)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusUnauthorized,
					Header: base.Header{
						"WWW-Authenticate": v.Header(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				err = v.ValidateRequest(req, nil)
				require.NoError(t, err)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mustMarshalSDP(medias.Marshal(false)),
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
					Protocol:    headers.TransportProtocolUDP,
					ServerPorts: &[2]int{34556, 34557},
					ClientPorts: inTH.ClientPorts,
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			func() {
				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mustMarshalSDP(medias.Marshal(false)),
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				v := auth.NewValidator("myuser", "mypass", nil)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusUnauthorized,
					Header: base.Header{
						"WWW-Authenticate": v.Header(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

				err = v.ValidateRequest(req, nil)
				require.NoError(t, err)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
					Protocol:       headers.TransportProtocolTCP,
					InterleavedIDs: inTH.InterleavedIDs,
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 0,
					Payload: testRTPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()
		}()

		packetRecv := make(chan struct{})

		c := Client{
			ReadTimeout: 1 * time.Second,
		}

		err = readAll(&c, "rtsp://myuser:mypass@localhost:8554/teststream",
			func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
				close(packetRecv)
			})
		require.NoError(t, err)
		defer c.Close()

		<-packetRecv
	})
}

func TestClientPlayDifferentInterleavedIDs(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		medias := media.Medias{testH264Media}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{2, 3},
		}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 2,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	packetRecv := make(chan struct{})

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
	}

	err = readAll(&c, "rtsp://localhost:8554/teststream",
		func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
			close(packetRecv)
		})
	require.NoError(t, err)
	defer c.Close()

	<-packetRecv
}

func TestClientPlayRedirect(t *testing.T) {
	for _, withCredentials := range []bool{false, true} {
		runName := "WithoutCredentials"
		if withCredentials {
			runName = "WithCredentials"
		}
		t.Run(runName, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				func() {
					nconn, err := l.Accept()
					require.NoError(t, err)
					defer nconn.Close()
					conn := conn.NewConn(nconn)

					req, err := conn.ReadRequest()
					require.NoError(t, err)
					require.Equal(t, base.Options, req.Method)

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
								string(base.Setup),
								string(base.Play),
							}, ", ")},
						},
					})
					require.NoError(t, err)

					req, err = conn.ReadRequest()
					require.NoError(t, err)
					require.Equal(t, base.Describe, req.Method)

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusMovedPermanently,
						Header: base.Header{
							"Location": base.HeaderValue{"rtsp://localhost:8554/test"},
						},
					})
					require.NoError(t, err)
				}()

				func() {
					nconn, err := l.Accept()
					require.NoError(t, err)
					defer nconn.Close()
					conn := conn.NewConn(nconn)

					req, err := conn.ReadRequest()
					require.NoError(t, err)
					require.Equal(t, base.Options, req.Method)

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
								string(base.Setup),
								string(base.Play),
							}, ", ")},
						},
					})

					require.NoError(t, err)

					req, err = conn.ReadRequest()
					require.NoError(t, err)
					require.Equal(t, base.Describe, req.Method)

					if withCredentials {
						if _, exists := req.Header["Authorization"]; !exists {
							authRealm := "example@localhost"
							authNonce := "exampleNonce"
							authOpaque := "exampleOpaque"
							authStale := "FALSE"
							authAlg := "MD5"
							err = conn.WriteResponse(&base.Response{
								Header: base.Header{
									"WWW-Authenticate": headers.Authenticate{
										Method:    headers.AuthDigest,
										Realm:     &authRealm,
										Nonce:     &authNonce,
										Opaque:    &authOpaque,
										Stale:     &authStale,
										Algorithm: &authAlg,
									}.Marshal(),
								},
								StatusCode: base.StatusUnauthorized,
							})
							require.NoError(t, err)
						}
						req, err = conn.ReadRequest()
						require.NoError(t, err)
						authHeaderVal, exists := req.Header["Authorization"]
						require.True(t, exists)
						var authHeader headers.Authenticate
						require.NoError(t, authHeader.Unmarshal(authHeaderVal))
						require.Equal(t, *authHeader.Username, "testusr")
						require.Equal(t, base.Describe, req.Method)
					}

					medias := media.Medias{testH264Media}
					medias.SetControls()

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
							"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
						},
						Body: mustMarshalSDP(medias.Marshal(false)),
					})
					require.NoError(t, err)

					req, err = conn.ReadRequest()
					require.NoError(t, err)
					require.Equal(t, base.Setup, req.Method)

					var th headers.Transport
					err = th.Unmarshal(req.Header["Transport"])
					require.NoError(t, err)

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Transport": headers.Transport{
								Protocol: headers.TransportProtocolUDP,
								Delivery: func() *headers.TransportDelivery {
									v := headers.TransportDeliveryUnicast
									return &v
								}(),
								ClientPorts: th.ClientPorts,
								ServerPorts: &[2]int{34556, 34557},
							}.Marshal(),
						},
					})
					require.NoError(t, err)

					req, err = conn.ReadRequest()
					require.NoError(t, err)
					require.Equal(t, base.Play, req.Method)

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
					})
					require.NoError(t, err)

					time.Sleep(500 * time.Millisecond)

					l1, err := net.ListenPacket("udp", "localhost:34556")
					require.NoError(t, err)
					defer l1.Close()

					l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})
				}()
			}()

			packetRecv := make(chan struct{})

			c := Client{}

			ru := "rtsp://localhost:8554/path1"
			if withCredentials {
				ru = "rtsp://testusr:testpwd@localhost:8554/path1"
			}
			err = readAll(&c, ru,
				func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
					close(packetRecv)
				})
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv
		})
	}
}

func TestClientPlayPause(t *testing.T) {
	writeFrames := func(inTH *headers.Transport, conn *conn.Conn) (chan struct{}, chan struct{}) {
		writerTerminate := make(chan struct{})
		writerDone := make(chan struct{})

		go func() {
			defer close(writerDone)

			var l1 net.PacketConn
			if inTH.Protocol == headers.TransportProtocolUDP {
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
					if inTH.Protocol == headers.TransportProtocolUDP {
						l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: inTH.ClientPorts[0],
						})
					} else {
						conn.WriteInterleavedFrame(&base.InterleavedFrame{
							Channel: 0,
							Payload: testRTPPacketMarshaled,
						}, make([]byte, 1024))
					}

				case <-writerTerminate:
					return
				}
			}
		}()

		return writerTerminate, writerDone
	}

	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				medias := media.Medias{testH264Media}
				medias.SetControls()

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mustMarshalSDP(medias.Marshal(false)),
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				writerTerminate, writerDone := writeFrames(&inTH, conn)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				close(writerTerminate)
				<-writerDone

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				writerTerminate, writerDone = writeFrames(&inTH, conn)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				close(writerTerminate)
				<-writerDone

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			firstFrame := int32(0)
			packetRecv := make(chan struct{})

			c := Client{
				Transport: func() *Transport {
					if transport == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
			}

			err = readAll(&c, "rtsp://localhost:8554/teststream",
				func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
					if atomic.SwapInt32(&firstFrame, 1) == 0 {
						close(packetRecv)
					}
				})
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv

			_, err = c.Pause()
			require.NoError(t, err)

			firstFrame = int32(0)
			packetRecv = make(chan struct{})

			_, err = c.Play(nil)
			require.NoError(t, err)

			<-packetRecv
		})
	}
}

func TestClientPlayRTCPReport(t *testing.T) {
	reportReceived := make(chan struct{})

	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		medias := media.Medias{testH264Media}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		l1, err := net.ListenPacket("udp", "localhost:27556")
		require.NoError(t, err)
		defer l1.Close()

		l2, err := net.ListenPacket("udp", "localhost:27557")
		require.NoError(t, err)
		defer l2.Close()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol: headers.TransportProtocolUDP,
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
					ServerPorts: &[2]int{27556, 27557},
					ClientPorts: inTH.ClientPorts,
				}.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		// skip firewall opening
		buf := make([]byte, 2048)
		_, _, err = l2.ReadFrom(buf)
		require.NoError(t, err)

		pkt := rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 946,
				Timestamp:      54352,
				SSRC:           753621,
			},
			Payload: []byte{0x05, 0x02, 0x03, 0x04},
		}
		byts, _ := pkt.Marshal()
		_, err = l1.WriteTo(byts, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[0],
		})
		require.NoError(t, err)

		// wait for the packet's SSRC to be saved
		time.Sleep(500 * time.Millisecond)

		sr := &rtcp.SenderReport{
			SSRC:        753621,
			NTPTime:     0,
			RTPTime:     0,
			PacketCount: 1,
			OctetCount:  4,
		}
		byts, _ = sr.Marshal()
		_, err = l2.WriteTo(byts, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[1],
		})
		require.NoError(t, err)

		buf = make([]byte, 2048)
		n, _, err := l2.ReadFrom(buf)
		require.NoError(t, err)
		packets, err := rtcp.Unmarshal(buf[:n])
		require.NoError(t, err)
		rr, ok := packets[0].(*rtcp.ReceiverReport)
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

		close(reportReceived)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	c := Client{
		udpReceiverReportPeriod: 1 * time.Second,
	}

	err = readAll(&c, "rtsp://localhost:8554/teststream", nil)
	require.NoError(t, err)
	defer c.Close()

	<-reportReceived
}

func TestClientPlayErrorTimeout(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
		"auto",
	} {
		t.Run(transport, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				medias := media.Medias{testH264Media}
				medias.SetControls()

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mustMarshalSDP(medias.Marshal(false)),
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
				}

				var l1 net.PacketConn
				if transport == "udp" || transport == "auto" {
					var err error
					l1, err = net.ListenPacket("udp", "localhost:34556")
					require.NoError(t, err)
					defer l1.Close()

					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				if transport == "udp" || transport == "auto" {
					// write a packet to skip the protocol autodetection feature
					l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})
				}

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			c := Client{
				Transport: func() *Transport {
					switch transport {
					case "udp":
						v := TransportUDP
						return &v

					case "tcp":
						v := TransportTCP
						return &v
					}
					return nil
				}(),
				InitialUDPReadTimeout: 1 * time.Second,
				ReadTimeout:           1 * time.Second,
			}

			err = readAll(&c, "rtsp://localhost:8554/teststream", nil)
			require.NoError(t, err)

			err = c.Wait()

			switch transport {
			case "udp", "auto":
				require.EqualError(t, err, "UDP timeout")

			case "tcp":
				require.EqualError(t, err, "TCP timeout")
			}
		})
	}
}

func TestClientPlayIgnoreTCPInvalidMedia(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		medias := media.Medias{testH264Media}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
		}
		th.Protocol = headers.TransportProtocolTCP
		th.InterleavedIDs = inTH.InterleavedIDs

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 6,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err)

		err = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	recv := make(chan struct{})

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
	}

	err = readAll(&c, "rtsp://localhost:8554/teststream",
		func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
			close(recv)
		})
	require.NoError(t, err)
	defer c.Close()

	<-recv
}

func TestClientPlaySeek(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		medias := media.Medias{testH264Media}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		var ra headers.Range
		err = ra.Unmarshal(req.Header["Range"])
		require.NoError(t, err)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: 5500 * time.Millisecond,
			},
		}, ra)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Pause, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = ra.Unmarshal(req.Header["Range"])
		require.NoError(t, err)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: 6400 * time.Millisecond,
			},
		}, ra)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
	}

	u, err := url.Parse("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	medias, baseURL, _, err := c.Describe(u)
	require.NoError(t, err)

	err = c.SetupAll(medias, baseURL)
	require.NoError(t, err)

	_, err = c.Play(&headers.Range{
		Value: &headers.RangeNPT{
			Start: 5500 * time.Millisecond,
		},
	})
	require.NoError(t, err)

	_, err = c.Seek(&headers.Range{
		Value: &headers.RangeNPT{
			Start: 6400 * time.Millisecond,
		},
	})
	require.NoError(t, err)
}

func TestClientPlayKeepaliveFromSession(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	keepaliveOk := make(chan struct{})

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		medias := media.Medias{testH264Media}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol: headers.TransportProtocolUDP,
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
					ClientPorts: inTH.ClientPorts,
					ServerPorts: &[2]int{34556, 34557},
				}.Marshal(),
				"Session": headers.Session{
					Session: "ABCDE",
					Timeout: func() *uint {
						v := uint(1)
						return &v
					}(),
				}.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		recv := make(chan struct{})
		go func() {
			defer close(recv)
			req, err = conn.ReadRequest()
			require.NoError(t, err)
			require.Equal(t, base.Options, req.Method)

			err = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
			})
			require.NoError(t, err)
		}()

		select {
		case <-recv:
		case <-time.After(3 * time.Second):
			t.Errorf("should not happen")
		}

		close(keepaliveOk)
	}()

	c := Client{}

	err = readAll(&c, "rtsp://localhost:8554/teststream", nil)
	require.NoError(t, err)
	defer c.Close()

	<-keepaliveOk
}

func TestClientPlayDifferentSource(t *testing.T) {
	packetRecv := make(chan struct{})

	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value"), req.URL)

		medias := media.Medias{testH264Media}
		medias.SetControls()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/test/stream?param=value/"},
			},
			Body: mustMarshalSDP(medias.Marshal(false)),
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"+medias[0].Control), req.URL)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		th := headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: inTH.ClientPorts,
			ServerPorts: &[2]int{34556, 34557},
			Source: func() *net.IP {
				i := net.ParseIP("127.0.1.1")
				return &i
			}(),
		}

		l1, err := net.ListenPacket("udp", "127.0.1.1:34556")
		require.NoError(t, err)
		defer l1.Close()

		l2, err := net.ListenPacket("udp", "127.0.1.1:34557")
		require.NoError(t, err)
		defer l2.Close()

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)

		// server -> client (RTP)
		l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: th.ClientPorts[0],
		})

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"), req.URL)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err)
	}()

	c := Client{
		Transport: func() *Transport {
			v := TransportUDP
			return &v
		}(),
	}

	err = readAll(&c, "rtsp://localhost:8554/test/stream?param=value",
		func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
			require.Equal(t, &testRTPPacket, pkt)
			close(packetRecv)
		})
	require.NoError(t, err)
	defer c.Close()

	<-packetRecv
}

func TestClientPlayDecodeErrors(t *testing.T) {
	for _, ca := range []struct {
		proto string
		name  string
	}{
		{"udp", "rtp invalid"},
		{"udp", "rtcp invalid"},
		{"udp", "rtp packets lost"},
		{"udp", "rtp too big"},
		{"udp", "rtcp too big"},
		{"tcp", "rtcp invalid"},
		{"tcp", "rtcp too big"},
	} {
		t.Run(ca.proto+" "+ca.name, func(t *testing.T) {
			errorRecv := make(chan struct{})

			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				medias := media.Medias{&media.Media{
					Type: media.TypeApplication,
					Formats: []format.Format{&format.Generic{
						PayloadTyp: 97,
						RTPMap:     "private/90000",
					}},
				}}
				medias.SetControls()

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/stream/"},
					},
					Body: mustMarshalSDP(medias.Marshal(false)),
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery: func() *headers.TransportDelivery {
						v := headers.TransportDeliveryUnicast
						return &v
					}(),
				}

				if ca.proto == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ClientPorts = inTH.ClientPorts
					th.ServerPorts = &[2]int{34556, 34557}
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				var l1 net.PacketConn
				var l2 net.PacketConn

				if ca.proto == "udp" {
					l1, err = net.ListenPacket("udp", "127.0.0.1:34556")
					require.NoError(t, err)
					defer l1.Close()

					l2, err = net.ListenPacket("udp", "127.0.0.1:34557")
					require.NoError(t, err)
					defer l2.Close()
				}

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err)

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)

				switch { //nolint:dupl
				case ca.proto == "udp" && ca.name == "rtp invalid":
					l1.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})

				case ca.proto == "udp" && ca.name == "rtcp invalid":
					l2.WriteTo([]byte{0x01, 0x02}, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[1],
					})

				case ca.proto == "udp" && ca.name == "rtp packets lost":
					byts, _ := rtp.Packet{
						Header: rtp.Header{
							PayloadType:    97,
							SequenceNumber: 30,
						},
					}.Marshal()
					l1.WriteTo(byts, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})

					byts, _ = rtp.Packet{
						Header: rtp.Header{
							PayloadType:    97,
							SequenceNumber: 100,
						},
					}.Marshal()
					l1.WriteTo(byts, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})

				case ca.proto == "udp" && ca.name == "rtp too big":
					l1.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})

				case ca.proto == "udp" && ca.name == "rtcp too big":
					l2.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[1],
					})

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

				req, err = conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				err = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err)
			}()

			c := Client{
				Transport: func() *Transport {
					if ca.proto == "udp" {
						v := TransportUDP
						return &v
					}
					v := TransportTCP
					return &v
				}(),
				OnDecodeError: func(err error) {
					switch {
					case ca.proto == "udp" && ca.name == "rtp invalid":
						require.EqualError(t, err, "RTP header size insufficient: 2 < 4")

					case ca.proto == "udp" && ca.name == "rtcp invalid":
						require.EqualError(t, err, "rtcp: packet too short")

					case ca.proto == "udp" && ca.name == "rtp packets lost":
						require.EqualError(t, err, "69 RTP packet(s) lost")

					case ca.proto == "udp" && ca.name == "rtp too big":
						require.EqualError(t, err, "RTP packet is too big to be read with UDP")

					case ca.proto == "udp" && ca.name == "rtcp too big":
						require.EqualError(t, err, "RTCP packet is too big to be read with UDP")

					case ca.proto == "tcp" && ca.name == "rtcp invalid":
						require.EqualError(t, err, "rtcp: packet too short")

					case ca.proto == "tcp" && ca.name == "rtcp too big":
						require.EqualError(t, err, "RTCP packet size (2000) is greater than maximum allowed (1472)")
					}
					close(errorRecv)
				},
			}

			err = readAll(&c, "rtsp://localhost:8554/stream", nil)
			require.NoError(t, err)
			defer c.Close()

			<-errorRecv
		})
	}
}
