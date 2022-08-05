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
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/pkg/auth"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/mpeg4audio"
	"github.com/aler9/gortsplib/pkg/url"
)

func startReading(c *Client, ur string) error {
	u, err := url.Parse(ur)
	if err != nil {
		return err
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		c.Close()
		return err
	}

	err = c.SetupAndPlay(tracks, baseURL)
	if err != nil {
		c.Close()
		return err
	}

	return nil
}

func TestClientReadTracks(t *testing.T) {
	track1 := &TrackH264{
		PayloadType: 96,
		SPS:         []byte{0x01, 0x02, 0x03, 0x04},
		PPS:         []byte{0x01, 0x02, 0x03, 0x04},
	}

	track2 := &TrackMPEG4Audio{
		PayloadType: 96,
		Config: &mpeg4audio.Config{
			Type:         2,
			SampleRate:   44100,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

	track3 := &TrackMPEG4Audio{
		PayloadType: 96,
		Config: &mpeg4audio.Config{
			Type:         2,
			SampleRate:   96000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

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
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		tracks := Tracks{track1, track2, track3}
		tracks.setControls()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			req, err := readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL(fmt.Sprintf("rtsp://localhost:8554/teststream/trackID=%d", i)), req.URL)

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

			byts, _ := base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Marshal(),
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)
		}

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	c := Client{}

	err = startReading(&c, "rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	defer c.Close()

	require.Equal(t, Tracks{track1, track2, track3}, c.Tracks())
}

func TestClientRead(t *testing.T) {
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

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value"), req.URL)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value"), req.URL)

				track := &TrackGeneric{
					Media:   "application",
					Formats: []string{"97"},
					RTPMap:  "97 private/90000",
				}

				tracks := Tracks{track}
				tracks.setControls()

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{scheme + "://" + listenIP + ":8554/test/stream?param=value/"},
					},
					Body: tracks.Marshal(false),
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value/trackID=0"), req.URL)

				var inTH headers.Transport
				err = inTH.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				th := headers.Transport{}

				var l1 net.PacketConn
				var l2 net.PacketConn

				switch transport {
				case "udp":
					v := headers.TransportDeliveryUnicast
					th.Delivery = &v
					th.Protocol = headers.TransportProtocolUDP
					th.ClientPorts = inTH.ClientPorts
					th.ServerPorts = &[2]int{34556, 34557}

					l1, err = net.ListenPacket("udp", listenIP+":34556")
					require.NoError(t, err)
					defer l1.Close()

					l2, err = net.ListenPacket("udp", listenIP+":34557")
					require.NoError(t, err)
					defer l2.Close()

				case "multicast":
					v := headers.TransportDeliveryMulticast
					th.Delivery = &v
					th.Protocol = headers.TransportProtocolUDP
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
					v := headers.TransportDeliveryUnicast
					th.Delivery = &v
					th.Protocol = headers.TransportProtocolTCP
					th.InterleavedIDs = &[2]int{0, 1}
				}

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value/"), req.URL)
				require.Equal(t, base.HeaderValue{"npt=0-"}, req.Header["Range"])

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				// server -> client (RTP)
				switch transport {
				case "udp":
					l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})

				case "multicast":
					l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("224.1.0.1"),
						Port: 25000,
					})

				case "tcp", "tls":
					byts, _ := base.InterleavedFrame{
						Channel: 0,
						Payload: testRTPPacketMarshaled,
					}.Marshal()
					_, err = conn.Write(byts)
					require.NoError(t, err)
				}

				// client -> server (RTCP)
				switch transport {
				case "udp", "multicast":
					// skip firewall opening
					if transport == "udp" {
						buf := make([]byte, 2048)
						_, _, err := l2.ReadFrom(buf)
						require.NoError(t, err)
					}

					buf := make([]byte, 2048)
					n, _, err := l2.ReadFrom(buf)
					require.NoError(t, err)
					packets, err := rtcp.Unmarshal(buf[:n])
					require.NoError(t, err)
					require.Equal(t, &testRTCPPacket, packets[0])
					close(packetRecv)

				case "tcp", "tls":
					var f base.InterleavedFrame
					err := f.Read(2048, br)
					require.NoError(t, err)
					require.Equal(t, 1, f.Channel)
					packets, err := rtcp.Unmarshal(f.Payload)
					require.NoError(t, err)
					require.Equal(t, &testRTCPPacket, packets[0])
					close(packetRecv)
				}

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL(scheme+"://"+listenIP+":8554/test/stream?param=value/"), req.URL)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
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

			c.OnPacketRTP = func(ctx *ClientOnPacketRTPCtx) {
				require.Equal(t, 0, ctx.TrackID)
				require.Equal(t, &testRTPPacket, ctx.Packet)

				err := c.WritePacketRTCP(0, &testRTCPPacket)
				require.NoError(t, err)
			}

			err = startReading(&c, scheme+"://"+listenIP+":8554/test/stream?param=value")
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv
		})
	}
}

func TestClientReadPartial(t *testing.T) {
	listenIP := multicastCapableIP(t)
	l, err := net.Listen("tcp", listenIP+":8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream"), req.URL)

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

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://" + listenIP + ":8554/teststream/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/trackID=1"), req.URL)

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

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		byts, _ = base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://"+listenIP+":8554/teststream/"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	packetRecv := make(chan struct{})

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
		OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
			require.Equal(t, 0, ctx.TrackID)
			require.Equal(t, &testRTPPacket, ctx.Packet)
			close(packetRecv)
		},
	}

	u, err := url.Parse("rtsp://" + listenIP + ":8554/teststream")
	require.NoError(t, err)

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	tracks, baseURL, _, err := c.Describe(u)
	require.NoError(t, err)

	_, err = c.Setup(true, tracks[1], baseURL, 0, 0)
	require.NoError(t, err)

	_, err = c.Play(nil)
	require.NoError(t, err)

	<-packetRecv
}

func TestClientReadContentBase(t *testing.T) {
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

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				track := &TrackH264{
					PayloadType: 96,
					SPS:         []byte{0x01, 0x02, 0x03, 0x04},
					PPS:         []byte{0x01, 0x02, 0x03, 0x04},
				}

				tracks := Tracks{track}
				tracks.setControls()

				switch ca {
				case "absent":
					byts, _ = base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
						},
						Body: tracks.Marshal(false),
					}.Marshal()
					_, err = conn.Write(byts)
					require.NoError(t, err)

				case "inside control attribute":
					body := string(tracks.Marshal(false))
					body = strings.Replace(body, "t=0 0", "t=0 0\r\na=control:rtsp://localhost:8554/teststream", 1)

					byts, _ = base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Content-Type": base.HeaderValue{"application/sdp"},
							"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream2/"},
						},
						Body: []byte(body),
					}.Marshal()
					_, err = conn.Write(byts)
					require.NoError(t, err)
				}

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

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

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)
				require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)
			}()

			c := Client{}

			err = startReading(&c, "rtsp://localhost:8554/teststream")
			require.NoError(t, err)
			c.Close()
		})
	}
}

func TestClientReadAnyPort(t *testing.T) {
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

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track := &TrackH264{
					PayloadType: 96,
					SPS:         []byte{0x01, 0x02, 0x03, 0x04},
					PPS:         []byte{0x01, 0x02, 0x03, 0x04},
				}

				tracks := Tracks{track}
				tracks.setControls()

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: tracks.Marshal(false),
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
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

				byts, _ = base.Response{
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
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
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
				OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
					require.Equal(t, &testRTPPacket, ctx.Packet)
					close(packetRecv)
				},
			}

			err = startReading(&c, "rtsp://localhost:8554/teststream")
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv

			if ca == "random" {
				c.WritePacketRTCP(0, &testRTCPPacket)
				<-serverRecv
			}
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
			br := bufio.NewReader(conn)

			req, err := readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Options, req.Method)

			byts, _ := base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Public": base.HeaderValue{strings.Join([]string{
						string(base.Describe),
						string(base.Setup),
						string(base.Play),
					}, ", ")},
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			tracks := Tracks{track}
			tracks.setControls()

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
					"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
				},
				Body: tracks.Marshal(false),
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusUnsupportedTransport,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			var inTH headers.Transport
			err = inTH.Unmarshal(req.Header["Transport"])
			require.NoError(t, err)
			require.Equal(t, headers.TransportProtocolTCP, inTH.Protocol)

			byts, _ = base.Response{
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
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Play, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			byts, _ = base.InterleavedFrame{
				Channel: 0,
				Payload: testRTPPacketMarshaled,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)
		}()

		packetRecv := make(chan struct{})

		c := Client{
			OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
				close(packetRecv)
			},
		}

		err = startReading(&c, "rtsp://localhost:8554/teststream")
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

			conn, err := l.Accept()
			require.NoError(t, err)
			br := bufio.NewReader(conn)

			req, err := readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Options, req.Method)

			byts, _ := base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Public": base.HeaderValue{strings.Join([]string{
						string(base.Describe),
						string(base.Setup),
						string(base.Play),
					}, ", ")},
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			v := auth.NewValidator("myuser", "mypass", nil)

			byts, _ = base.Response{
				StatusCode: base.StatusUnauthorized,
				Header: base.Header{
					"WWW-Authenticate": v.Header(),
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			err = v.ValidateRequest(req)
			require.NoError(t, err)

			track := &TrackH264{
				PayloadType: 96,
				SPS:         []byte{0x01, 0x02, 0x03, 0x04},
				PPS:         []byte{0x01, 0x02, 0x03, 0x04},
			}

			tracks := Tracks{track}
			tracks.setControls()

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
					"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
				},
				Body: tracks.Marshal(false),
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

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

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Marshal(),
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Play, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Teardown, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			conn.Close()

			conn, err = l.Accept()
			require.NoError(t, err)
			br = bufio.NewReader(conn)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Options, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Public": base.HeaderValue{strings.Join([]string{
						string(base.Describe),
						string(base.Setup),
						string(base.Play),
					}, ", ")},
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Describe, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"application/sdp"},
					"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
				},
				Body: tracks.Marshal(false),
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)

			v = auth.NewValidator("myuser", "mypass", nil)

			byts, _ = base.Response{
				StatusCode: base.StatusUnauthorized,
				Header: base.Header{
					"WWW-Authenticate": v.Header(),
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Setup, req.Method)
			require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

			err = v.ValidateRequest(req)
			require.NoError(t, err)

			inTH = headers.Transport{}
			err = inTH.Unmarshal(req.Header["Transport"])
			require.NoError(t, err)

			th = headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Protocol:       headers.TransportProtocolTCP,
				InterleavedIDs: inTH.InterleavedIDs,
			}

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": th.Marshal(),
				},
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Play, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			byts, _ = base.InterleavedFrame{
				Channel: 0,
				Payload: testRTPPacketMarshaled,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Teardown, req.Method)

			byts, _ = base.Response{
				StatusCode: base.StatusOK,
			}.Marshal()
			_, err = conn.Write(byts)
			require.NoError(t, err)

			conn.Close()
		}()

		packetRecv := make(chan struct{})

		c := Client{
			ReadTimeout: 1 * time.Second,
			OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
				close(packetRecv)
			},
		}

		err = startReading(&c, "rtsp://myuser:mypass@localhost:8554/teststream")
		require.NoError(t, err)
		defer c.Close()

		<-packetRecv
	})
}

func TestClientReadDifferentInterleavedIDs(t *testing.T) {
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
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		track1 := &TrackH264{
			PayloadType: 96,
			SPS:         []byte{0x01, 0x02, 0x03, 0x04},
			PPS:         []byte{0x01, 0x02, 0x03, 0x04},
		}

		tracks := Tracks{track1}
		tracks.setControls()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/trackID=0"), req.URL)

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

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		byts, _ = base.InterleavedFrame{
			Channel: 2,
			Payload: testRTPPacketMarshaled,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream/"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	packetRecv := make(chan struct{})

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
		OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
			require.Equal(t, 0, ctx.TrackID)
			close(packetRecv)
		},
	}

	err = startReading(&c, "rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	defer c.Close()

	<-packetRecv
}

func TestClientReadRedirect(t *testing.T) {
	for _, withCredentials := range []bool{false, true} {
		runName := "WithoutCredentials"
		if withCredentials {
			runName = "WithCredentials"
		}
		t.Run(runName, func(t *testing.T) {
			packetRecv := make(chan struct{})

			c := Client{
				OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
					close(packetRecv)
				},
			}

			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				conn, err := l.Accept()
				require.NoError(t, err)
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusMovedPermanently,
					Header: base.Header{
						"Location": base.HeaderValue{"rtsp://localhost:8554/test"},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				conn.Close()

				conn, err = l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br = bufio.NewReader(conn)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Marshal()

				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				if withCredentials {
					if _, exists := req.Header["Authorization"]; !exists {
						authRealm := "example@localhost"
						authNonce := "exampleNonce"
						authOpaque := "exampleOpaque"
						authStale := "FALSE"
						authAlg := "MD5"
						byts, _ = base.Response{
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
						}.Marshal()
						_, err = conn.Write(byts)
						require.NoError(t, err)
					}
					req, err = readRequest(br)
					require.NoError(t, err)
					authHeaderVal, exists := req.Header["Authorization"]
					require.True(t, exists)
					var authHeader headers.Authenticate
					require.NoError(t, authHeader.Unmarshal(authHeaderVal))
					require.Equal(t, *authHeader.Username, "testusr")
					require.Equal(t, base.Describe, req.Method)
				}

				track := &TrackH264{
					PayloadType: 96,
					SPS:         []byte{0x01, 0x02, 0x03, 0x04},
					PPS:         []byte{0x01, 0x02, 0x03, 0x04},
				}

				tracks := Tracks{track}
				tracks.setControls()

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: tracks.Marshal(false),
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Setup, req.Method)

				var th headers.Transport
				err = th.Unmarshal(req.Header["Transport"])
				require.NoError(t, err)

				byts, _ = base.Response{
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
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
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

			ru := "rtsp://localhost:8554/path1"
			if withCredentials {
				ru = "rtsp://testusr:testpwd@localhost:8554/path1"
			}
			err = startReading(&c, ru)
			require.NoError(t, err)
			defer c.Close()

			<-packetRecv
		})
	}
}

func TestClientReadPause(t *testing.T) {
	writeFrames := func(inTH *headers.Transport, conn net.Conn, br *bufio.Reader) (chan struct{}, chan struct{}) {
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
						byts, _ := base.InterleavedFrame{
							Channel: 0,
							Payload: testRTPPacketMarshaled,
						}.Marshal()
						conn.Write(byts)
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

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track := &TrackH264{
					PayloadType: 96,
					SPS:         []byte{0x01, 0x02, 0x03, 0x04},
					PPS:         []byte{0x01, 0x02, 0x03, 0x04},
				}

				tracks := Tracks{track}
				tracks.setControls()

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: tracks.Marshal(false),
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
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

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				writerTerminate, writerDone := writeFrames(&inTH, conn, br)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				close(writerTerminate)
				<-writerDone

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				writerTerminate, writerDone = writeFrames(&inTH, conn, br)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				close(writerTerminate)
				<-writerDone

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
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
				OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
					if atomic.SwapInt32(&firstFrame, 1) == 0 {
						close(packetRecv)
					}
				},
			}

			err = startReading(&c, "rtsp://localhost:8554/teststream")
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

func TestClientReadRTCPReport(t *testing.T) {
	reportReceived := make(chan struct{})

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
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track := &TrackH264{
			PayloadType: 96,
			SPS:         []byte{0x01, 0x02, 0x03, 0x04},
			PPS:         []byte{0x01, 0x02, 0x03, 0x04},
		}

		tracks := Tracks{track}
		tracks.setControls()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
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

		byts, _ = base.Response{
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
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
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
		byts, _ = pkt.Marshal()
		_, err = l1.WriteTo(byts, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: inTH.ClientPorts[0],
		})
		require.NoError(t, err)

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

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	c := Client{
		udpReceiverReportPeriod: 1 * time.Second,
	}

	err = startReading(&c, "rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	defer c.Close()

	<-reportReceived
}

func TestClientReadErrorTimeout(t *testing.T) {
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

				conn, err := l.Accept()
				require.NoError(t, err)
				defer conn.Close()
				br := bufio.NewReader(conn)

				req, err := readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				byts, _ := base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
							string(base.Setup),
							string(base.Play),
						}, ", ")},
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Describe, req.Method)

				track := &TrackH264{
					PayloadType: 96,
					SPS:         []byte{0x01, 0x02, 0x03, 0x04},
					PPS:         []byte{0x01, 0x02, 0x03, 0x04},
				}

				tracks := Tracks{track}
				tracks.setControls()

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp"},
						"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
					},
					Body: tracks.Marshal(false),
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
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

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Transport": th.Marshal(),
					},
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Play, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
				require.NoError(t, err)

				if transport == "udp" || transport == "auto" {
					// write a packet to skip the protocol autodetection feature
					l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: th.ClientPorts[0],
					})
				}

				req, err = readRequest(br)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

				byts, _ = base.Response{
					StatusCode: base.StatusOK,
				}.Marshal()
				_, err = conn.Write(byts)
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

			err = startReading(&c, "rtsp://localhost:8554/teststream")
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
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track := &TrackH264{
			PayloadType: 96,
			SPS:         []byte{0x01, 0x02, 0x03, 0x04},
			PPS:         []byte{0x01, 0x02, 0x03, 0x04},
		}

		tracks := Tracks{track}
		tracks.setControls()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
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

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		byts, _ = base.InterleavedFrame{
			Channel: 6,
			Payload: testRTPPacketMarshaled,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		byts, _ = base.InterleavedFrame{
			Channel: 0,
			Payload: testRTPPacketMarshaled,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	recv := make(chan struct{})

	c := Client{
		Transport: func() *Transport {
			v := TransportTCP
			return &v
		}(),
		OnPacketRTP: func(ctx *ClientOnPacketRTPCtx) {
			close(recv)
		},
	}

	err = startReading(&c, "rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	defer c.Close()

	<-recv
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
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track := &TrackH264{
			PayloadType: 96,
			SPS:         []byte{0x01, 0x02, 0x03, 0x04},
			PPS:         []byte{0x01, 0x02, 0x03, 0x04},
		}

		tracks := Tracks{track}
		tracks.setControls()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
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

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		var ra headers.Range
		err = ra.Unmarshal(req.Header["Range"])
		require.NoError(t, err)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: headers.RangeNPTTime(5500 * time.Millisecond),
			},
		}, ra)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Pause, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		err = ra.Unmarshal(req.Header["Range"])
		require.NoError(t, err)
		require.Equal(t, headers.Range{
			Value: &headers.RangeNPT{
				Start: headers.RangeNPTTime(6400 * time.Millisecond),
			},
		}, ra)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
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

	tracks, baseURL, _, err := c.Describe(u)
	require.NoError(t, err)

	for _, track := range tracks {
		_, err := c.Setup(true, track, baseURL, 0, 0)
		require.NoError(t, err)
	}

	_, err = c.Play(&headers.Range{
		Value: &headers.RangeNPT{
			Start: headers.RangeNPTTime(5500 * time.Millisecond),
		},
	})
	require.NoError(t, err)

	_, err = c.Seek(&headers.Range{
		Value: &headers.RangeNPT{
			Start: headers.RangeNPTTime(6400 * time.Millisecond),
		},
	})
	require.NoError(t, err)
}

func TestClientReadKeepaliveFromSession(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	keepaliveOk := make(chan struct{})

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		track := &TrackH264{
			PayloadType: 96,
			SPS:         []byte{0x01, 0x02, 0x03, 0x04},
			PPS:         []byte{0x01, 0x02, 0x03, 0x04},
		}

		tracks := Tracks{track}
		tracks.setControls()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)

		var inTH headers.Transport
		err = inTH.Unmarshal(req.Header["Transport"])
		require.NoError(t, err)

		byts, _ = base.Response{
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
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		recv := make(chan struct{})
		go func() {
			defer close(recv)
			req, err = readRequest(br)
			require.NoError(t, err)
			require.Equal(t, base.Options, req.Method)

			byts, _ := base.Response{
				StatusCode: base.StatusOK,
			}.Marshal()
			_, err = conn.Write(byts)
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

	err = startReading(&c, "rtsp://localhost:8554/teststream")
	require.NoError(t, err)
	defer c.Close()

	<-keepaliveOk
}

func TestClientReadDifferentSource(t *testing.T) {
	packetRecv := make(chan struct{})

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
		br := bufio.NewReader(conn)

		req, err := readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value"), req.URL)

		byts, _ := base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Setup),
					string(base.Play),
				}, ", ")},
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value"), req.URL)

		track := &TrackH264{
			PayloadType: 96,
			SPS:         []byte{0x01, 0x02, 0x03, 0x04},
			PPS:         []byte{0x01, 0x02, 0x03, 0x04},
		}

		tracks := Tracks{track}
		tracks.setControls()

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/test/stream?param=value/"},
			},
			Body: tracks.Marshal(false),
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Setup, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/trackID=0"), req.URL)

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

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": th.Marshal(),
			},
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Play, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"), req.URL)
		require.Equal(t, base.HeaderValue{"npt=0-"}, req.Header["Range"])

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)

		// server -> client (RTP)
		time.Sleep(1 * time.Second)
		l1.WriteTo(testRTPPacketMarshaled, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: th.ClientPorts[0],
		})

		req, err = readRequest(br)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/test/stream?param=value/"), req.URL)

		byts, _ = base.Response{
			StatusCode: base.StatusOK,
		}.Marshal()
		_, err = conn.Write(byts)
		require.NoError(t, err)
	}()

	c := Client{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		Transport: func() *Transport {
			v := TransportUDP
			return &v
		}(),
	}

	c.OnPacketRTP = func(ctx *ClientOnPacketRTPCtx) {
		require.Equal(t, 0, ctx.TrackID)
		require.Equal(t, &testRTPPacket, ctx.Packet)
		close(packetRecv)
	}

	err = startReading(&c, "rtsp://localhost:8554/test/stream?param=value")
	require.NoError(t, err)
	defer c.Close()

	<-packetRecv
}
