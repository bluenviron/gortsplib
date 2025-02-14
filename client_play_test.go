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

	"github.com/bluenviron/gortsplib/v4/pkg/auth"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/conn"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
)

func ipPtr(v net.IP) *net.IP {
	return &v
}

func deliveryPtr(v headers.TransportDelivery) *headers.TransportDelivery {
	return &v
}

func transportPtr(v Transport) *Transport {
	return &v
}

func transportModePtr(v headers.TransportMode) *headers.TransportMode {
	return &v
}

func mediasToSDP(medias []*description.Media) []byte {
	desc := &description.Session{
		Medias: medias,
	}

	prepareForAnnounce(desc)

	byts, err := desc.Marshal(false)
	if err != nil {
		panic(err)
	}

	return byts
}

func mustMarshalPacketRTP(pkt *rtp.Packet) []byte {
	byts, err := pkt.Marshal()
	if err != nil {
		panic(err)
	}
	return byts
}

func mustMarshalPacketRTCP(pkt rtcp.Packet) []byte {
	byts, err := pkt.Marshal()
	if err != nil {
		panic(err)
	}
	return byts
}

func readAll(c *Client, ur string, cb func(*description.Media, format.Format, *rtp.Packet)) error {
	u, err := base.ParseURL(ur)
	if err != nil {
		return err
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	sd, _, err := c.Describe(u)
	if err != nil {
		c.Close()
		return err
	}

	err = c.SetupAll(sd.BaseURL, sd.Medias)
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

	media2 := &description.Media{
		Type: description.MediaTypeAudio,
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

	media3 := &description.Media{
		Type: description.MediaTypeAudio,
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		medias := []*description.Media{media1, media2, media3}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		for i := 0; i < 3; i++ {
			req, err2 = conn.ReadRequest()
			require.NoError(t, err2)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[i].Control), req.URL)

			var inTH headers.Transport
			err2 = inTH.Unmarshal(req.Header["Transport"])
			require.NoError(t, err2)

			th := headers.Transport{
				Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
				Protocol:    headers.TransportProtocolUDP,
				ClientPorts: inTH.ClientPorts,
				ServerPorts: &[2]int{34556 + i*2, 34557 + i*2},
			}

			err2 = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Marshal(),
				},
			})
			require.NoError(t, err2)
		}

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
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

				var cert tls.Certificate
				cert, err = tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)

				l = tls.NewListener(l, &tls.Config{Certificates: []tls.Certificate{cert}})
			} else {
				scheme = "rtsp"
			}

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value"), req.URL)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value"), req.URL)

				forma := &format.Generic{
					PayloadTyp: 96,
					RTPMa:      "private/90000",
				}
				err2 = forma.Init()
				require.NoError(t, err2)

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

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{scheme + "://" + listenIP + ":8554/test/stream?param=value/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				var l1s [2]net.PacketConn
				var l2s [2]net.PacketConn
				var clientPorts [2]*[2]int

				for i := 0; i < 2; i++ {
					req, err2 = conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Setup, req.Method)
					require.Equal(t, mustParseURL(
						scheme+"://"+listenIP+":8554/test/stream?param=value/"+medias[i].Control), req.URL)

					var inTH headers.Transport
					err2 = inTH.Unmarshal(req.Header["Transport"])
					require.NoError(t, err2)

					require.Equal(t, (*headers.TransportMode)(nil), inTH.Mode)

					var th headers.Transport

					switch transport {
					case "udp":
						v := headers.TransportDeliveryUnicast
						th.Delivery = &v
						th.Protocol = headers.TransportProtocolUDP
						th.ClientPorts = inTH.ClientPorts
						clientPorts[i] = inTH.ClientPorts
						th.ServerPorts = &[2]int{34556 + i*2, 34557 + i*2}

						l1s[i], err2 = net.ListenPacket(
							"udp", net.JoinHostPort(listenIP, strconv.FormatInt(int64(th.ServerPorts[0]), 10)))
						require.NoError(t, err2)
						defer l1s[i].Close()

						l2s[i], err2 = net.ListenPacket(
							"udp", net.JoinHostPort(listenIP, strconv.FormatInt(int64(th.ServerPorts[1]), 10)))
						require.NoError(t, err2)
						defer l2s[i].Close()

					case "multicast":
						v := headers.TransportDeliveryMulticast
						th.Delivery = &v
						th.Protocol = headers.TransportProtocolUDP
						v2 := net.ParseIP("224.1.0.1")
						th.Destination = &v2
						th.Ports = &[2]int{25000 + i*2, 25001 + i*2}

						l1s[i], err2 = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[0]), 10))
						require.NoError(t, err2)
						defer l1s[i].Close()

						p := ipv4.NewPacketConn(l1s[i])

						var intfs []net.Interface
						intfs, err2 = net.Interfaces()
						require.NoError(t, err2)

						for _, intf := range intfs {
							err2 = p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP("224.1.0.1")})
							require.NoError(t, err2)
						}

						l2s[i], err2 = net.ListenPacket("udp", "224.0.0.0:25001")
						require.NoError(t, err2)
						defer l2s[i].Close()

						p = ipv4.NewPacketConn(l2s[i])

						intfs, err2 = net.Interfaces()
						require.NoError(t, err2)

						for _, intf := range intfs {
							err2 = p.JoinGroup(&intf, &net.UDPAddr{IP: net.ParseIP("224.1.0.1")})
							require.NoError(t, err2)
						}

					case "tcp", "tls":
						v := headers.TransportDeliveryUnicast
						th.Delivery = &v
						th.Protocol = headers.TransportProtocolTCP
						th.InterleavedIDs = &[2]int{0 + i*2, 1 + i*2}
					}

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Transport": th.Marshal(),
						},
					})
					require.NoError(t, err2)
				}

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value/"), req.URL)
				require.Equal(t, base.HeaderValue{"npt=0-"}, req.Header["Range"])

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				for i := 0; i < 2; i++ {
					// server -> client (RTP)
					switch transport {
					case "udp":
						_, err2 = l1s[i].WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: clientPorts[i][0],
						})
						require.NoError(t, err2)

					case "multicast":
						_, err2 = l1s[i].WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
							IP:   net.ParseIP("224.1.0.1"),
							Port: 25000,
						})
						require.NoError(t, err2)

					case "tcp", "tls":
						err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
							Channel: 0 + i*2,
							Payload: testRTPPacketMarshaled,
						}, make([]byte, 1024))
						require.NoError(t, err2)
					}

					// client -> server (RTCP)
					switch transport {
					case "udp", "multicast":
						// skip firewall opening
						if transport == "udp" {
							buf := make([]byte, 2048)
							_, _, err2 = l2s[i].ReadFrom(buf)
							require.NoError(t, err2)
						}

						buf := make([]byte, 2048)
						var n int
						n, _, err2 = l2s[i].ReadFrom(buf)
						require.NoError(t, err2)
						var packets []rtcp.Packet
						packets, err2 = rtcp.Unmarshal(buf[:n])
						require.NoError(t, err2)
						require.Equal(t, &testRTCPPacket, packets[0])

					case "tcp", "tls":
						var f *base.InterleavedFrame
						f, err2 = conn.ReadInterleavedFrame()
						require.NoError(t, err2)
						require.Equal(t, 1+i*2, f.Channel)
						var packets []rtcp.Packet
						packets, err2 = rtcp.Unmarshal(f.Payload)
						require.NoError(t, err2)
						require.Equal(t, &testRTCPPacket, packets[0])
					}
				}

				close(packetRecv)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value/"), req.URL)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)
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

			u, err := base.ParseURL(scheme + "://" + listenIP + ":8554/test/stream?param=value")
			require.NoError(t, err)

			err = c.Start(u.Scheme, u.Host)
			require.NoError(t, err)
			defer c.Close()

			sd, _, err := c.Describe(u)
			require.NoError(t, err)

			err = c.SetupAll(sd.BaseURL, sd.Medias)
			require.NoError(t, err)

			c.OnPacketRTPAny(func(medi *description.Media, _ format.Format, pkt *rtp.Packet) {
				require.Equal(t, &testRTPPacket, pkt)
				err2 := c.WritePacketRTCP(medi, &testRTCPPacket)
				require.NoError(t, err2)
			})

			_, err = c.Play(nil)
			require.NoError(t, err)

			<-packetRecv

			s := c.Stats()
			require.Greater(t, s.Session.BytesSent, uint64(19))
			require.Less(t, s.Session.BytesSent, uint64(41))
			require.Greater(t, s.Session.BytesReceived, uint64(31))
			require.Less(t, s.Session.BytesReceived, uint64(37))
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream"), req.URL)

		forma := &format.Generic{
			PayloadTyp: 96,
			RTPMa:      "private/90000",
		}
		err2 = forma.Init()
		require.NoError(t, err2)

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

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://" + listenIP + ":8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"+medias[1].Control), req.URL)

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)
		require.Equal(t, &[2]int{0, 1}, inTH.InterleavedIDs)

		th := headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	packetRecv := make(chan struct{})

	c := Client{
		Transport: transportPtr(TransportTCP),
	}

	u, err := base.ParseURL("rtsp://" + listenIP + ":8554/teststream")
	require.NoError(t, err)

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	sd, _, err := c.Describe(u)
	require.NoError(t, err)

	_, err = c.Setup(sd.BaseURL, sd.Medias[1], 0, 0)
	require.NoError(t, err)

	c.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		require.Equal(t, sd.Medias[1], medi)
		require.Equal(t, sd.Medias[1].Formats[0], forma)
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

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				medias := []*description.Media{testH264Media}

				switch ca {
				case "absent":
					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
						},
						Body: mediasToSDP(medias),
					})
					require.NoError(t, err2)

				case "inside control attribute":
					body := string(mediasToSDP(medias))
					body = strings.Replace(body, "t=0 0", "t=0 0\r\na=control:rtsp://localhost:8554/teststream", 1)

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
							"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream2/"},
						},
						Body: []byte(body),
					})
					require.NoError(t, err2)
				}

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

				var inTH headers.Transport
				err2 = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)

				th := headers.Transport{
					Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
					Protocol:    headers.TransportProtocolUDP,
					ClientPorts: inTH.ClientPorts,
					ServerPorts: &[2]int{34556, 34557},
				}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)
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

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				medias := []*description.Media{testH264Media}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				var th headers.Transport
				err2 = th.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)

				l1a, err2 := net.ListenPacket("udp", "localhost:13344")
				require.NoError(t, err2)
				defer l1a.Close()

				l1b, err2 := net.ListenPacket("udp", "localhost:23041")
				require.NoError(t, err2)
				defer l1b.Close()

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": headers.Transport{
							Protocol:    headers.TransportProtocolUDP,
							Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
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
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				time.Sleep(500 * time.Millisecond)

				_, err2 = l1a.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ClientPorts[0],
				})
				require.NoError(t, err2)

				// read RTCP
				if ca == "random" {
					// skip firewall opening
					buf := make([]byte, 2048)
					_, _, err = l1b.ReadFrom(buf)
					require.NoError(t, err)

					buf = make([]byte, 2048)
					var n int
					n, _, err = l1b.ReadFrom(buf)
					require.NoError(t, err)
					var packets []rtcp.Packet
					packets, err = rtcp.Unmarshal(buf[:n])
					require.NoError(t, err)
					require.Equal(t, &testRTCPPacket, packets[0])
					close(serverRecv)
				}
			}()

			packetRecv := make(chan struct{})

			c := Client{
				AnyPortEnable: true,
			}

			var med *description.Media
			err = readAll(&c, "rtsp://localhost:8554/teststream",
				func(medi *description.Media, _ format.Format, pkt *rtp.Packet) {
					require.Equal(t, &testRTPPacket, pkt)
					med = medi
					close(packetRecv)
				})
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv

			if ca == "random" {
				err := c.WritePacketRTCP(med, &testRTCPPacket)
				require.NoError(t, err)
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

			nconn, err2 := l.Accept()
			require.NoError(t, err2)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			req, err2 := conn.ReadRequest()
			require.NoError(t, err2)
			require.Equal(t, base.Options, req.Method)

			err2 = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Public": base.HeaderValue{strings.Join([]string{
						string(base.Describe),
						string(base.Setup),
						string(base.Play),
					}, ", ")},
				},
			})
			require.NoError(t, err2)

			req, err2 = conn.ReadRequest()
			require.NoError(t, err2)
			require.Equal(t, base.Describe, req.Method)

			medias := []*description.Media{testH264Media}

			err2 = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
					"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
				},
				Body: mediasToSDP(medias),
			})
			require.NoError(t, err2)

			req, err2 = conn.ReadRequest()
			require.NoError(t, err2)
			require.Equal(t, base.Setup, req.Method)

			err2 = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusUnsupportedTransport,
			})
			require.NoError(t, err2)

			req, err2 = conn.ReadRequest()
			require.NoError(t, err2)
			require.Equal(t, base.Setup, req.Method)

			var inTH headers.Transport
			err2 = inTH.Unmarshal(req.Header["Transport"])
			require.NoError(t, err2)
			require.Equal(t, headers.TransportProtocolTCP, inTH.Protocol)

			err2 = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": headers.Transport{
						Protocol:       headers.TransportProtocolTCP,
						Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
						InterleavedIDs: &[2]int{0, 1},
					}.Marshal(),
				},
			})
			require.NoError(t, err2)

			req, err2 = conn.ReadRequest()
			require.NoError(t, err2)
			require.Equal(t, base.Play, req.Method)

			err2 = conn.WriteResponse(&base.Response{
				StatusCode: base.StatusOK,
			})
			require.NoError(t, err2)

			err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
				Channel: 0,
				Payload: testRTPPacketMarshaled,
			}, make([]byte, 1024))
			require.NoError(t, err2)
		}()

		msgRecv := make(chan struct{})
		packetRecv := make(chan struct{})

		c := Client{
			OnTransportSwitch: func(err error) {
				require.EqualError(t, err, "switching to TCP because server requested it")
				close(msgRecv)
			},
		}

		err = readAll(&c, "rtsp://localhost:8554/teststream",
			func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
				close(packetRecv)
			})
		require.NoError(t, err)
		defer c.Close()

		<-msgRecv
		<-packetRecv
	})

	t.Run("switch after tcp response", func(t *testing.T) {
		l, err := net.Listen("tcp", "localhost:8554")
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()
		go func() {
			defer close(serverDone)

			medias := []*description.Media{testH264Media}

			func() {
				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				co := conn.NewConn(nconn)

				req, err2 := co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": headers.Transport{
							Protocol:       headers.TransportProtocolTCP,
							Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
							InterleavedIDs: &[2]int{0, 1},
							ServerPorts:    &[2]int{12312, 12313},
						}.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				_, err2 = co.ReadRequest()
				require.Error(t, err2)
			}()

			func() {
				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				co := conn.NewConn(nconn)

				req, err2 := co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err2 = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)
				require.Equal(t, headers.TransportProtocolTCP, inTH.Protocol)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": headers.Transport{
							Protocol:       headers.TransportProtocolTCP,
							Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
							InterleavedIDs: &[2]int{0, 1},
						}.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = co.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = co.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				err2 = co.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 0,
					Payload: testRTPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err2)
			}()
		}()

		msgRecv := make(chan struct{})
		packetRecv := make(chan struct{})

		c := Client{
			OnTransportSwitch: func(err error) {
				require.EqualError(t, err, "switching to TCP because server requested it")
				close(msgRecv)
			},
		}

		err = readAll(&c, "rtsp://localhost:8554/teststream",
			func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
				close(packetRecv)
			})
		require.NoError(t, err)
		defer c.Close()

		<-msgRecv
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

			medias := []*description.Media{testH264Media}

			func() {
				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				nonce, err2 := auth.GenerateNonce()
				require.NoError(t, err2)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusUnauthorized,
					Header: base.Header{
						"WWW-Authenticate": auth.GenerateWWWAuthenticate(nil, "IPCAM", nonce),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				err2 = auth.Verify(req, "myuser", "mypass", nil, "IPCAM", nonce)
				require.NoError(t, err2)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{
					Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
					Protocol:    headers.TransportProtocolUDP,
					ServerPorts: &[2]int{34556, 34557},
					ClientPorts: inTH.ClientPorts,
				}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)
			}()

			func() {
				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				nonce, err2 := auth.GenerateNonce()
				require.NoError(t, err2)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusUnauthorized,
					Header: base.Header{
						"WWW-Authenticate": auth.GenerateWWWAuthenticate(nil, "IPCAM", nonce),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

				err2 = auth.Verify(req, "myuser", "mypass", nil, "IPCAM", nonce)
				require.NoError(t, err2)

				var inTH headers.Transport
				err2 = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)

				th := headers.Transport{
					Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
					Protocol:       headers.TransportProtocolTCP,
					InterleavedIDs: inTH.InterleavedIDs,
				}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 0,
					Payload: testRTPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)
			}()
		}()

		msgRecv := make(chan struct{})
		packetRecv := make(chan struct{})

		c := Client{
			OnTransportSwitch: func(err error) {
				require.EqualError(t, err, "no UDP packets received, switching to TCP")
				close(msgRecv)
			},
			ReadTimeout: 1 * time.Second,
		}

		err = readAll(&c, "rtsp://myuser:mypass@localhost:8554/teststream",
			func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
				close(packetRecv)
			})
		require.NoError(t, err)
		defer c.Close()

		<-msgRecv
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"+medias[0].Control), req.URL)

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		th := headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{1, 2}, // use odd value
		}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 1,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	packetRecv := make(chan struct{})

	c := Client{
		Transport: transportPtr(TransportTCP),
	}

	err = readAll(&c, "rtsp://localhost:8554/teststream",
		func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
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
					var nconn net.Conn
					nconn, err = l.Accept()
					require.NoError(t, err)
					defer nconn.Close()
					conn := conn.NewConn(nconn)

					req, err2 := conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Options, req.Method)

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
								string(base.Setup),
								string(base.Play),
							}, ", ")},
						},
					})
					require.NoError(t, err2)

					req, err2 = conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Describe, req.Method)

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusMovedPermanently,
						Header: base.Header{
							"Location": base.HeaderValue{"rtsp://localhost:8554/test"},
						},
					})
					require.NoError(t, err2)
				}()

				func() {
					nconn, err2 := l.Accept()
					require.NoError(t, err2)
					defer nconn.Close()
					conn := conn.NewConn(nconn)

					req, err2 := conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Options, req.Method)

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
								string(base.Setup),
								string(base.Play),
							}, ", ")},
						},
					})

					require.NoError(t, err2)

					req, err2 = conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Describe, req.Method)

					if withCredentials {
						if _, exists := req.Header["Authorization"]; !exists {
							authRealm := "example@localhost"
							authNonce := "exampleNonce"
							authOpaque := "exampleOpaque"
							authStale := "FALSE"

							err2 = conn.WriteResponse(&base.Response{
								Header: base.Header{
									"WWW-Authenticate": headers.Authenticate{
										Method: headers.AuthMethodDigest,
										Realm:  authRealm,
										Nonce:  authNonce,
										Opaque: &authOpaque,
										Stale:  &authStale,
									}.Marshal(),
								},
								StatusCode: base.StatusUnauthorized,
							})
							require.NoError(t, err2)
						}

						req, err2 = conn.ReadRequest()
						require.NoError(t, err2)

						authHeaderVal, exists := req.Header["Authorization"]
						require.True(t, exists)

						var authHeader headers.Authorization
						require.NoError(t, authHeader.Unmarshal(authHeaderVal))
						require.Equal(t, authHeader.Username, "testusr")
						require.Equal(t, base.Describe, req.Method)
					}

					medias := []*description.Media{testH264Media}

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
							"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
						},
						Body: mediasToSDP(medias),
					})
					require.NoError(t, err2)

					req, err2 = conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Setup, req.Method)

					var th headers.Transport
					err2 = th.Unmarshal(req.Header["Transport"])
					require.NoError(t, err2)

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Transport": headers.Transport{
								Protocol:    headers.TransportProtocolUDP,
								Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
								ClientPorts: th.ClientPorts,
								ServerPorts: &[2]int{34556, 34557},
							}.Marshal(),
						},
					})
					require.NoError(t, err2)

					req, err2 = conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Play, req.Method)

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
					})
					require.NoError(t, err2)

					time.Sleep(500 * time.Millisecond)

					l1, err2 := net.ListenPacket("udp", "localhost:34556")
					require.NoError(t, err2)
					defer l1.Close()

					_, err2 = l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})
					require.NoError(t, err2)
				}()
			}()

			packetRecv := make(chan struct{})

			c := Client{}

			ru := "rtsp://localhost:8554/path1"
			if withCredentials {
				ru = "rtsp://testusr:testpwd@localhost:8554/path1"
			}
			err = readAll(&c, ru,
				func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
					close(packetRecv)
				})
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv
		})
	}
}

func TestClientPlayPausePlay(t *testing.T) {
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

			ti := time.NewTicker(50 * time.Millisecond)
			defer ti.Stop()

			for {
				select {
				case <-ti.C:
					if inTH.Protocol == headers.TransportProtocolUDP {
						_, err := l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: inTH.ClientPorts[0],
						})
						require.NoError(t, err)
					} else {
						err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
							Channel: 0,
							Payload: testRTPPacketMarshaled,
						}, make([]byte, 1024))
						require.NoError(t, err)
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

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				medias := []*description.Media{testH264Media}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err2 = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				if transport == "udp" {
					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				writerTerminate, writerDone := writeFrames(&inTH, conn)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Pause, req.Method)

				close(writerTerminate)
				<-writerDone

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				writerTerminate, writerDone = writeFrames(&inTH, conn)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)

				close(writerTerminate)
				<-writerDone

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)
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
				func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		l1, err2 := net.ListenPacket("udp", "localhost:27556")
		require.NoError(t, err2)
		defer l1.Close()

		l2, err2 := net.ListenPacket("udp", "localhost:27557")
		require.NoError(t, err2)
		defer l2.Close()

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol:    headers.TransportProtocolUDP,
					Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
					ServerPorts: &[2]int{27556, 27557},
					ClientPorts: inTH.ClientPorts,
				}.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		// skip firewall opening
		buf := make([]byte, 2048)
		_, _, err2 = l2.ReadFrom(buf)
		require.NoError(t, err2)

		_, err2 = l1.WriteTo(mustMarshalPacketRTP(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 946,
				Timestamp:      54352,
				SSRC:           753621,
			},
			Payload: []byte{0x05, 0x02, 0x03, 0x04},
		}), &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[0],
		})
		require.NoError(t, err2)

		// wait for the packet's SSRC to be saved
		time.Sleep(200 * time.Millisecond)

		_, err2 = l2.WriteTo(mustMarshalPacketRTCP(&rtcp.SenderReport{
			SSRC:        753621,
			NTPTime:     ntpTimeGoToRTCP(time.Date(2017, 8, 12, 15, 30, 0, 0, time.UTC)),
			RTPTime:     54352,
			PacketCount: 1,
			OctetCount:  4,
		}), &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[1],
		})
		require.NoError(t, err2)

		buf = make([]byte, 2048)
		n, _, err2 := l2.ReadFrom(buf)
		require.NoError(t, err2)
		packets, err2 := rtcp.Unmarshal(buf[:n])
		require.NoError(t, err2)
		rr, ok := packets[0].(*rtcp.ReceiverReport)
		require.True(t, ok)
		require.Equal(t, &rtcp.ReceiverReport{
			SSRC: rr.SSRC,
			Reports: []rtcp.ReceptionReport{
				{
					SSRC:               rr.Reports[0].SSRC,
					LastSequenceNumber: 946,
					LastSenderReport:   2641887232,
					Delay:              rr.Reports[0].Delay,
				},
			},
			ProfileExtensions: []uint8{},
		}, rr)

		close(reportReceived)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	c := Client{
		receiverReportPeriod: 500 * time.Millisecond,
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

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				medias := []*description.Media{testH264Media}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err2 = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
				}

				var l1 net.PacketConn
				if transport == "udp" || transport == "auto" {
					l1, err2 = net.ListenPacket("udp", "localhost:34556")
					require.NoError(t, err2)
					defer l1.Close()

					th.Protocol = headers.TransportProtocolUDP
					th.ServerPorts = &[2]int{34556, 34557}
					th.ClientPorts = inTH.ClientPorts
				} else {
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = inTH.InterleavedIDs
				}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				if transport == "auto" {
					// write a packet to skip the protocol autodetection feature
					_, err2 = l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})
					require.NoError(t, err2)
				}

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)

		medias := []*description.Media{testH264Media}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		th := headers.Transport{
			Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
		}
		th.Protocol = headers.TransportProtocolTCP
		th.InterleavedIDs = inTH.InterleavedIDs

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 6,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err2)

		err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	recv := make(chan struct{})

	c := Client{
		Transport: transportPtr(TransportTCP),
	}

	err = readAll(&c, "rtsp://localhost:8554/teststream",
		func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		th := headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)

		var ra headers.Range
		err2 = ra.Unmarshal(req.Header["Range"])
		require.NoError(t, err2)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: 5500 * time.Millisecond,
			},
		}, ra)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Pause, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)

		err2 = ra.Unmarshal(req.Header["Range"])
		require.NoError(t, err2)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: 6400 * time.Millisecond,
			},
		}, ra)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	c := Client{
		Transport: transportPtr(TransportTCP),
	}

	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	sd, _, err := c.Describe(u)
	require.NoError(t, err)

	err = c.SetupAll(sd.BaseURL, sd.Medias)
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

func TestClientPlayKeepalive(t *testing.T) {
	for _, ca := range []string{"response before frame", "response after frame", "no response"} {
		t.Run(ca, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"CSeq": req.Header["CSeq"],
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				medias := []*description.Media{testH264Media}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"CSeq":         req.Header["CSeq"],
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err2 = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"CSeq": req.Header["CSeq"],
						"Transport": headers.Transport{
							Protocol:       headers.TransportProtocolTCP,
							Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
							InterleavedIDs: &[2]int{0, 1},
						}.Marshal(),
						"Session": headers.Session{
							Session: "ABCDE",
							Timeout: uintPtr(1),
						}.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"CSeq": req.Header["CSeq"],
					},
				})
				require.NoError(t, err2)

				recv := make(chan struct{})
				go func() {
					defer close(recv)
					req, err2 = conn.ReadRequest()
					require.NoError(t, err2)
					require.Equal(t, base.Options, req.Method)
				}()

				select {
				case <-recv:
				case <-time.After(2 * time.Second):
					t.Errorf("should not happen")
				}

				if ca == "response before frame" {
					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"CSeq": req.Header["CSeq"],
						},
					})
					require.NoError(t, err2)
				}

				err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 0,
					Payload: testRTPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err2)

				if ca == "response after frame" {
					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
					})
					require.NoError(t, err2)
				}

				err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
					Channel: 0,
					Payload: testRTPPacketMarshaled,
				}, make([]byte, 1024))
				require.NoError(t, err2)
			}()

			done1 := make(chan struct{})
			done2 := make(chan struct{})
			n := 0
			m := 0

			v := TransportTCP
			c := Client{
				Transport: &v,
				OnResponse: func(_ *base.Response) {
					m++
					if ca != "no response" {
						if m >= 5 {
							close(done2)
						}
					} else {
						if m >= 4 {
							close(done2)
						}
					}
				},
			}

			err = readAll(&c, "rtsp://localhost:8554/teststream",
				func(_ *description.Media, _ format.Format, pkt *rtp.Packet) {
					require.Equal(t, &testRTPPacket, pkt)
					n++
					if n == 2 {
						close(done1)
					}
				})
			require.NoError(t, err)
			defer c.Close()

			<-done1
			<-done2
		})
	}
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value"), req.URL)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/test/stream?param=value/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"+medias[0].Control), req.URL)

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		th := headers.Transport{
			Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: inTH.ClientPorts,
			ServerPorts: &[2]int{34556, 34557},
			Source:      ipPtr(net.ParseIP("127.0.1.1")),
		}

		l1, err2 := net.ListenPacket("udp", "127.0.1.1:34556")
		require.NoError(t, err2)
		defer l1.Close()

		l2, err2 := net.ListenPacket("udp", "127.0.1.1:34557")
		require.NoError(t, err2)
		defer l2.Close()

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		// server -> client (RTP)
		_, err2 = l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: th.ClientPorts[0],
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"), req.URL)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	c := Client{
		Transport: transportPtr(TransportUDP),
	}

	err = readAll(&c, "rtsp://localhost:8554/test/stream?param=value",
		func(_ *description.Media, _ format.Format, pkt *rtp.Packet) {
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
		{"udp", "rtp unknown payload type"},
		{"udp", "wrong ssrc"},
		{"udp", "rtcp too big"},
		{"udp", "rtp too big"},
		{"tcp", "rtp invalid"},
		{"tcp", "rtcp invalid"},
		{"tcp", "rtp unknown payload type"},
		{"tcp", "wrong ssrc"},
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

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)

				medias := []*description.Media{{
					Type: description.MediaTypeApplication,
					Formats: []format.Format{&format.Generic{
						PayloadTyp: 97,
						RTPMa:      "private/90000",
					}},
				}}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/stream/"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Setup, req.Method)

				var inTH headers.Transport
				err2 = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err2)

				th := headers.Transport{
					Delivery: deliveryPtr(headers.TransportDeliveryUnicast),
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
					l1, err2 = net.ListenPacket("udp", "127.0.0.1:34556")
					require.NoError(t, err2)
					defer l1.Close()

					l2, err2 = net.ListenPacket("udp", "127.0.0.1:34557")
					require.NoError(t, err2)
					defer l2.Close()
				}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Play, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)

				var writeRTP func(buf []byte)
				var writeRTCP func(byts []byte)

				if ca.proto == "udp" { //nolint:dupl
					writeRTP = func(byts []byte) {
						_, err2 = l1.WriteTo(byts, &net.UDPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: th.ClientPorts[0],
						})
						require.NoError(t, err2)
					}

					writeRTCP = func(byts []byte) {
						_, err2 = l2.WriteTo(byts, &net.UDPAddr{
							IP:   net.ParseIP("127.0.0.1"),
							Port: th.ClientPorts[1],
						})
						require.NoError(t, err2)
					}
				} else {
					writeRTP = func(byts []byte) {
						err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
							Channel: 0,
							Payload: byts,
						}, make([]byte, 2048))
						require.NoError(t, err2)
					}

					writeRTCP = func(byts []byte) {
						err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
							Channel: 1,
							Payload: byts,
						}, make([]byte, 2048))
						require.NoError(t, err2)
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
					_, err2 = l1.WriteTo(bytes.Repeat([]byte{0x01, 0x02}, 2000/2), &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})
					require.NoError(t, err2)
				}

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Teardown, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
				})
				require.NoError(t, err2)
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
				OnPacketLost: func(err error) {
					require.EqualError(t, err, "69 RTP packets lost")
					close(errorRecv)
				},
				OnDecodeError: func(err error) {
					switch {
					case ca.name == "rtp invalid":
						require.EqualError(t, err, "RTP header size insufficient: 2 < 4")

					case ca.name == "rtcp invalid":
						require.EqualError(t, err, "rtcp: packet too short")

					case ca.name == "rtp unknown payload type":
						require.EqualError(t, err, "received RTP packet with unknown payload type: 111")

					case ca.name == "wrong ssrc":
						require.EqualError(t, err, "received packet with wrong SSRC 456, expected 123")

					case ca.proto == "udp" && ca.name == "rtcp too big":
						require.EqualError(t, err, "RTCP packet is too big to be read with UDP")

					case ca.proto == "tcp" && ca.name == "rtcp too big":
						require.EqualError(t, err, "RTCP packet size (2000) is greater than maximum allowed (1472)")

					case ca.proto == "udp" && ca.name == "rtp too big":
						require.EqualError(t, err, "RTP packet is too big to be read with UDP")

					default:
						t.Errorf("unexpected")
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

func TestClientPlayPacketNTP(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		l1, err2 := net.ListenPacket("udp", "localhost:27556")
		require.NoError(t, err2)
		defer l1.Close()

		l2, err2 := net.ListenPacket("udp", "localhost:27557")
		require.NoError(t, err2)
		defer l2.Close()

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": headers.Transport{
					Protocol:    headers.TransportProtocolUDP,
					Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
					ServerPorts: &[2]int{27556, 27557},
					ClientPorts: inTH.ClientPorts,
				}.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		// skip firewall opening
		buf := make([]byte, 2048)
		_, _, err2 = l2.ReadFrom(buf)
		require.NoError(t, err2)

		_, err2 = l1.WriteTo(mustMarshalPacketRTP(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 946,
				Timestamp:      54352,
				SSRC:           753621,
			},
			Payload: []byte{1, 2, 3, 4},
		}), &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[0],
		})
		require.NoError(t, err2)

		// wait for the packet's SSRC to be saved
		time.Sleep(100 * time.Millisecond)

		_, err2 = l2.WriteTo(mustMarshalPacketRTCP(&rtcp.SenderReport{
			SSRC:        753621,
			NTPTime:     ntpTimeGoToRTCP(time.Date(2017, 8, 12, 15, 30, 0, 0, time.UTC)),
			RTPTime:     54352,
			PacketCount: 1,
			OctetCount:  4,
		}), &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[1],
		})
		require.NoError(t, err2)

		time.Sleep(100 * time.Millisecond)

		_, err2 = l1.WriteTo(mustMarshalPacketRTP(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 947,
				Timestamp:      54352 + 90000,
				SSRC:           753621,
			},
			Payload: []byte{5, 6, 7, 8},
		}), &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[0],
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	c := Client{}

	recv := make(chan struct{})
	first := false

	err = readAll(&c, "rtsp://localhost:8554/teststream",
		func(medi *description.Media, _ format.Format, pkt *rtp.Packet) {
			if !first {
				first = true
			} else {
				ntp, ok := c.PacketNTP(medi, pkt)
				require.Equal(t, true, ok)
				require.Equal(t, time.Date(2017, 8, 12, 15, 30, 1, 0, time.UTC), ntp.UTC())
				close(recv)
			}
		})
	require.NoError(t, err)
	defer c.Close()

	<-recv
}

func TestClientPlayBackChannel(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()

	go func() {
		defer close(serverDone)

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, base.HeaderValue{"www.onvif.org/ver20/backchannel"}, req.Header["Require"])

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP([]*description.Media{
				testH264Media,
				{
					Type: description.MediaTypeAudio,
					Formats: []format.Format{&format.G711{
						PayloadTyp:   8,
						MULaw:        false,
						SampleRate:   8000,
						ChannelCount: 1,
					}},
					IsBackChannel: true,
				},
			}),
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, base.HeaderValue(nil), req.Header["Require"])

		var inTH headers.Transport
		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		th := headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, base.HeaderValue{"www.onvif.org/ver20/backchannel"}, req.Header["Require"])

		err2 = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err2)

		th = headers.Transport{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: inTH.InterleavedIDs,
		}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, base.HeaderValue{"www.onvif.org/ver20/backchannel"}, req.Header["Require"])

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)

		f, err2 := conn.ReadInterleavedFrame()
		require.NoError(t, err2)
		require.Equal(t, 2, f.Channel)

		f, err2 = conn.ReadInterleavedFrame()
		require.NoError(t, err2)
		require.Equal(t, 3, f.Channel)

		packets, err2 := rtcp.Unmarshal(f.Payload)
		require.NoError(t, err2)

		sr, ok := packets[0].(*rtcp.SenderReport)
		require.Equal(t, true, ok)
		require.Equal(t, uint32(0x38F27A2F), sr.SSRC)
		require.Equal(t, uint32(1), sr.PacketCount)
		require.Equal(t, uint32(4), sr.OctetCount)

		err2 = conn.WriteInterleavedFrame(&base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}, make([]byte, 1024))
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, base.HeaderValue{"www.onvif.org/ver20/backchannel"}, req.Header["Require"])

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
		})
		require.NoError(t, err2)
	}()

	c := Client{
		RequestBackChannels:  true,
		Transport:            transportPtr(TransportTCP),
		senderReportPeriod:   500 * time.Millisecond,
		receiverReportPeriod: 750 * time.Millisecond,
	}

	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	sd, _, err := c.Describe(u)
	require.NoError(t, err)

	err = c.SetupAll(sd.BaseURL, sd.Medias)
	require.NoError(t, err)

	recv := make(chan struct{})

	c.OnPacketRTP(sd.Medias[0], sd.Medias[0].Formats[0], func(_ *rtp.Packet) {
		close(recv)
	})

	_, err = c.Play(nil)
	require.NoError(t, err)

	err = c.WritePacketRTP(sd.Medias[1], &rtp.Packet{
		Header: rtp.Header{
			Version:     2,
			PayloadType: 8,
			CSRC:        []uint32{},
			SSRC:        0x38F27A2F,
		},
		Payload: []byte{1, 2, 3, 4},
	})
	require.NoError(t, err)

	<-recv
}
