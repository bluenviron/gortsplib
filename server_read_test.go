package gortsplib

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/rtp"
	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/ipv4"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

func multicastCapableIP(t *testing.T) string {
	intfs, err := net.Interfaces()
	require.NoError(t, err)

	for _, intf := range intfs {
		if (intf.Flags & net.FlagMulticast) != 0 {
			addrs, err := intf.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					return v.IP.String()
				case *net.IPAddr:
					return v.IP.String()
				}
			}
		}
	}

	t.Errorf("unable to find a multicast IP")
	return ""
}

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
			track, err := NewTrackH264(96, &TrackConfigH264{
				[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
			})
			require.NoError(t, err)

			stream := NewServerStream(Tracks{track, track, track, track, track})

			s := &Server{
				Handler: &testServerHandler{
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						require.Equal(t, ca.path, ctx.Path)
						require.Equal(t, ca.trackID, ctx.TrackID)
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
				},
				RTSPAddress: "localhost:8554",
			}

			err = s.Start()
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			br := bufio.NewReader(conn)

			th := &headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{ca.trackID * 2, (ca.trackID * 2) + 1},
			}

			res, err := writeReqReadRes(conn, br, base.Request{
				Method: base.Setup,
				URL:    mustParseURL(ca.url),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": th.Write(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerReadSetupErrors(t *testing.T) {
	for _, ca := range []string{"different paths", "double setup"} {
		t.Run(ca, func(t *testing.T) {
			connClosed := make(chan struct{})

			track, err := NewTrackH264(96, &TrackConfigH264{
				[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
			})
			require.NoError(t, err)

			stream := NewServerStream(Tracks{track})

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						if ca == "different paths" {
							require.EqualError(t, ctx.Error, "can't setup tracks with different paths")
						} else {
							require.EqualError(t, ctx.Error, "track 0 has already been setup")
						}
						close(connClosed)
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
				},
				RTSPAddress: "localhost:8554",
			}

			err = s.Start()
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			br := bufio.NewReader(conn)

			th := &headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}

			if ca == "different paths" {
				res, err := writeReqReadRes(conn, br, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"1"},
						"Transport": th.Write(),
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				th.InterleavedIDs = &[2]int{2, 3}

				res, err = writeReqReadRes(conn, br, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/test12stream/trackID=1"),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"2"},
						"Transport": th.Write(),
						"Session":   res.Header["Session"],
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)
			} else {
				res, err := writeReqReadRes(conn, br, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"1"},
						"Transport": th.Write(),
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				th.InterleavedIDs = &[2]int{2, 3}

				res, err = writeReqReadRes(conn, br, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
					Header: base.Header{
						"CSeq":      base.HeaderValue{"2"},
						"Transport": th.Write(),
						"Session":   res.Header["Session"],
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusBadRequest, res.StatusCode)
			}

			<-connClosed
		})
	}
}

func TestServerRead(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
		"tls",
		"multicast",
	} {
		t.Run(transport, func(t *testing.T) {
			connOpened := make(chan struct{})
			connClosed := make(chan struct{})
			sessionOpened := make(chan struct{})
			sessionClosed := make(chan struct{})
			framesReceived := make(chan struct{})

			track, err := NewTrackH264(96, &TrackConfigH264{
				[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
			})
			require.NoError(t, err)

			stream := NewServerStream(Tracks{track})

			counter := uint64(0)

			listenIP := multicastCapableIP(t)

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
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						go func() {
							time.Sleep(1 * time.Second)
							stream.WritePacketRTP(0, []byte{0x01, 0x02, 0x03, 0x04})
							stream.WritePacketRTCP(0, []byte{0x05, 0x06, 0x07, 0x08})
						}()

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onPacketRTCP: func(ctx *ServerHandlerOnPacketRTCPCtx) {
						// skip multicast loopback
						if transport == "multicast" && atomic.AddUint64(&counter, 1) <= 1 {
							return
						}

						require.Equal(t, 0, ctx.TrackID)
						require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, ctx.Payload)
						close(framesReceived)
					},
					onGetParameter: func(ctx *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress: listenIP + ":8554",
			}

			switch transport {
			case "udp":
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"

			case "multicast":
				s.MulticastIPRange = "224.1.0.0/16"
				s.MulticastRTPPort = 8000
				s.MulticastRTCPPort = 8001

			case "tls":
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			err = s.Start()
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", listenIP+":8554")
			require.NoError(t, err)

			conn = func() net.Conn {
				if transport == "tls" {
					return tls.Client(conn, &tls.Config{InsecureSkipVerify: true})
				}
				return conn
			}()
			br := bufio.NewReader(conn)
			var bb bytes.Buffer

			<-connOpened

			inTH := &headers.Transport{
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
			}

			switch transport {
			case "udp":
				v := headers.TransportDeliveryUnicast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}

			case "multicast":
				v := headers.TransportDeliveryMulticast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolUDP

			default:
				v := headers.TransportDeliveryUnicast
				inTH.Delivery = &v
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{4, 5}
			}

			res, err := writeReqReadRes(conn, br, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Write(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			var th headers.Transport
			err = th.Read(res.Header["Transport"])
			require.NoError(t, err)

			var l1 net.PacketConn
			var l2 net.PacketConn

			switch transport {
			case "udp":
				require.Equal(t, headers.TransportProtocolUDP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)

				l1, err = net.ListenPacket("udp", listenIP+":35466")
				require.NoError(t, err)
				defer l1.Close()

				l2, err = net.ListenPacket("udp", listenIP+":35467")
				require.NoError(t, err)
				defer l2.Close()

			case "multicast":
				require.Equal(t, headers.TransportProtocolUDP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryMulticast, *th.Delivery)

				l1, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[0]), 10))
				require.NoError(t, err)
				defer l1.Close()

				p := ipv4.NewPacketConn(l1)

				intfs, err := net.Interfaces()
				require.NoError(t, err)

				for _, intf := range intfs {
					err := p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
					require.NoError(t, err)
				}

				l2, err = net.ListenPacket("udp", "224.0.0.0:"+strconv.FormatInt(int64(th.Ports[1]), 10))
				require.NoError(t, err)
				defer l2.Close()

				p = ipv4.NewPacketConn(l2)

				intfs, err = net.Interfaces()
				require.NoError(t, err)

				for _, intf := range intfs {
					err := p.JoinGroup(&intf, &net.UDPAddr{IP: *th.Destination})
					require.NoError(t, err)
				}

			default:
				require.Equal(t, headers.TransportProtocolTCP, th.Protocol)
				require.Equal(t, headers.TransportDeliveryUnicast, *th.Delivery)
			}

			<-sessionOpened

			res, err = writeReqReadRes(conn, br, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			// server -> client
			if transport == "udp" || transport == "multicast" {
				buf := make([]byte, 2048)
				n, _, err := l1.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, buf[:n])

				// skip firewall opening
				if transport == "udp" {
					buf := make([]byte, 2048)
					_, _, err := l2.ReadFrom(buf)
					require.NoError(t, err)
				}

				buf = make([]byte, 2048)
				n, _, err = l2.ReadFrom(buf)
				require.NoError(t, err)
				require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, buf[:n])
			} else {
				var f base.InterleavedFrame
				f.Payload = make([]byte, 2048)
				err := f.Read(br)
				require.NoError(t, err)
				require.Equal(t, 4, f.Channel)
				require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, f.Payload)

				f.Payload = make([]byte, 2048)
				err = f.Read(br)
				require.NoError(t, err)
				require.Equal(t, 5, f.Channel)
				require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, f.Payload)
			}

			// client -> server (RTCP)
			switch transport {
			case "udp":
				l2.WriteTo([]byte{0x01, 0x02, 0x03, 0x04}, &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
				<-framesReceived

			case "multicast":
				l2.WriteTo([]byte{0x01, 0x02, 0x03, 0x04}, &net.UDPAddr{
					IP:   *th.Destination,
					Port: th.Ports[1],
				})
				<-framesReceived

			default:
				base.InterleavedFrame{
					Channel: 5,
					Payload: []byte{0x01, 0x02, 0x03, 0x04},
				}.Write(&bb)
				_, err = conn.Write(bb.Bytes())
				require.NoError(t, err)
				<-framesReceived
			}

			if transport == "udp" || transport == "multicast" {
				// ping with OPTIONS
				res, err = writeReqReadRes(conn, br, base.Request{
					Method: base.Options,
					URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"4"},
						"Session": res.Header["Session"],
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				// ping with GET_PARAMETER
				res, err = writeReqReadRes(conn, br, base.Request{
					Method: base.GetParameter,
					URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"5"},
						"Session": res.Header["Session"],
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)
			}

			res, err = writeReqReadRes(conn, br, base.Request{
				Method: base.Teardown,
				URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"6"},
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

func TestServerReadVLCMulticast(t *testing.T) {
	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	listenIP := multicastCapableIP(t)

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(ctx *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
		},
		RTSPAddress:       listenIP + ":8554",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8000,
		MulticastRTCPPort: 8001,
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", listenIP+":8554")
	require.NoError(t, err)
	br := bufio.NewReader(conn)
	defer conn.Close()

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Describe,
		URL:    mustParseURL("rtsp://" + listenIP + ":8554/teststream?vlcmulticast"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var desc psdp.SessionDescription
	err = desc.Unmarshal(res.Body)
	require.NoError(t, err)

	require.Equal(t, "224.1.0.0", desc.ConnectionInformation.Address.Address)
}

func TestServerReadNonStandardFrameSize(t *testing.T) {
	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	payload := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 4096/5)

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					time.Sleep(1 * time.Second)
					stream.WritePacketRTP(0, payload)
				}()

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	br := bufio.NewReader(conn)

	inTH := &headers.Transport{
		Mode: func() *headers.TransportMode {
			v := headers.TransportModePlay
			return &v
		}(),
		Delivery: func() *headers.TransportDelivery {
			v := headers.TransportDeliveryUnicast
			return &v
		}(),
		Protocol:       headers.TransportProtocolTCP,
		InterleavedIDs: &[2]int{0, 1},
	}

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"1"},
			"Transport": inTH.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var f base.InterleavedFrame
	f.Payload = make([]byte, 4500)
	err = f.Read(br)
	require.NoError(t, err)
	require.Equal(t, 0, f.Channel)
	require.Equal(t, payload, f.Payload)
}

func TestServerReadTCPResponseBeforeFrames(t *testing.T) {
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	s := &Server{
		RTSPAddress: "localhost:8554",
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					defer close(writerDone)

					stream.WritePacketRTP(0, []byte("\x00\x00\x00\x00"))

					t := time.NewTicker(50 * time.Millisecond)
					defer t.Stop()

					for {
						select {
						case <-t.C:
							stream.WritePacketRTP(0, []byte("\x00\x00\x00\x00"))
						case <-writerTerminate:
							return
						}
					}
				}()

				time.Sleep(50 * time.Millisecond)

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var fr base.InterleavedFrame
	fr.Payload = make([]byte, 2048)
	err = fr.Read(br)
	require.NoError(t, err)
}

func TestServerReadPlayPlay(t *testing.T) {
	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
		RTSPAddress:    "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolUDP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				ClientPorts: &[2]int{30450, 30451},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"3"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerReadPlayPausePlay(t *testing.T) {
	writerStarted := false
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				if !writerStarted {
					writerStarted = true
					go func() {
						defer close(writerDone)

						t := time.NewTicker(50 * time.Millisecond)
						defer t.Stop()

						for {
							select {
							case <-t.C:
								stream.WritePacketRTP(0, []byte("\x00\x00\x00\x00"))
							case <-writerTerminate:
								return
							}
						}
					}()
				}

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Pause,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerReadPlayPausePause(t *testing.T) {
	writerDone := make(chan struct{})
	writerTerminate := make(chan struct{})

	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				close(writerTerminate)
				<-writerDone
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					defer close(writerDone)

					t := time.NewTicker(50 * time.Millisecond)
					defer t.Stop()

					for {
						select {
						case <-t.C:
							stream.WritePacketRTP(0, []byte("\x00\x00\x00\x00"))
						case <-writerTerminate:
							return
						}
					}
				}()

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)
	var bb bytes.Buffer

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	base.Request{
		Method: base.Pause,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	}.Write(&bb)
	_, err = conn.Write(bb.Bytes())
	require.NoError(t, err)

	res, err = readResIgnoreFrames(br)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	base.Request{
		Method: base.Pause,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	}.Write(&bb)
	_, err = conn.Write(bb.Bytes())
	require.NoError(t, err)

	res, err = readResIgnoreFrames(br)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerReadTimeout(t *testing.T) {
	for _, transport := range []string{
		"udp",
		// there's no timeout when reading with TCP
	} {
		t.Run(transport, func(t *testing.T) {
			sessionClosed := make(chan struct{})

			track, err := NewTrackH264(96, &TrackConfigH264{
				[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
			})
			require.NoError(t, err)

			stream := NewServerStream(Tracks{track})

			s := &Server{
				Handler: &testServerHandler{
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				ReadTimeout:                    1 * time.Second,
				closeSessionAfterNoRequestsFor: 1 * time.Second,
				UDPRTPAddress:                  "127.0.0.1:8000",
				UDPRTCPAddress:                 "127.0.0.1:8001",
				RTSPAddress:                    "localhost:8554",
			}

			err = s.Start()
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			br := bufio.NewReader(conn)

			inTH := &headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
			}

			inTH.Protocol = headers.TransportProtocolUDP
			inTH.ClientPorts = &[2]int{35466, 35467}

			res, err := writeReqReadRes(conn, br, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Write(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			res, err = writeReqReadRes(conn, br, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			<-sessionClosed
		})
	}
}

func TestServerReadWithoutTeardown(t *testing.T) {
	for _, transport := range []string{
		"udp",
		"tcp",
	} {
		t.Run(transport, func(t *testing.T) {
			connClosed := make(chan struct{})
			sessionClosed := make(chan struct{})

			track, err := NewTrackH264(96, &TrackConfigH264{
				[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
			})
			require.NoError(t, err)

			stream := NewServerStream(Tracks{track})

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
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				ReadTimeout:                    1 * time.Second,
				closeSessionAfterNoRequestsFor: 1 * time.Second,
				RTSPAddress:                    "localhost:8554",
			}

			if transport == "udp" {
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"
			}

			err = s.Start()
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			br := bufio.NewReader(conn)

			inTH := &headers.Transport{
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
			}

			if transport == "udp" {
				inTH.Protocol = headers.TransportProtocolUDP
				inTH.ClientPorts = &[2]int{35466, 35467}
			} else {
				inTH.Protocol = headers.TransportProtocolTCP
				inTH.InterleavedIDs = &[2]int{0, 1}
			}

			res, err := writeReqReadRes(conn, br, base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Write(),
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			res, err = writeReqReadRes(conn, br, base.Request{
				Method: base.Play,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"2"},
					"Session": res.Header["Session"],
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			conn.Close()

			<-sessionClosed
			<-connClosed
		})
	}
}

func TestServerReadUDPChangeConn(t *testing.T) {
	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onGetParameter: func(ctx *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
		RTSPAddress:    "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	sxID := ""

	func() {
		conn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, err := writeReqReadRes(conn, br, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"1"},
				"Transport": inTH.Write(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		res, err = writeReqReadRes(conn, br, base.Request{
			Method: base.Play,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"2"},
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
		br := bufio.NewReader(conn)

		res, err := writeReqReadRes(conn, br, base.Request{
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

func TestServerReadPartialTracks(t *testing.T) {
	track1, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	track2, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track1, track2})

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					time.Sleep(1 * time.Second)
					stream.WritePacketRTP(0, []byte{0x01, 0x02, 0x03, 0x04})
					stream.WritePacketRTP(1, []byte{0x05, 0x06, 0x07, 0x08})
				}()

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	inTH := &headers.Transport{
		Delivery: func() *headers.TransportDelivery {
			v := headers.TransportDeliveryUnicast
			return &v
		}(),
		Mode: func() *headers.TransportMode {
			v := headers.TransportModePlay
			return &v
		}(),
		Protocol:       headers.TransportProtocolTCP,
		InterleavedIDs: &[2]int{4, 5},
	}

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=1"),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"1"},
			"Transport": inTH.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var f base.InterleavedFrame
	f.Payload = make([]byte, 2048)
	err = f.Read(br)
	require.NoError(t, err)
	require.Equal(t, 4, f.Channel)
	require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, f.Payload)
}

func TestServerReadAdditionalInfos(t *testing.T) {
	getInfos := func() (*headers.RTPInfo, []*uint32) {
		conn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		ssrcs := make([]*uint32, 2)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		}

		res, err := writeReqReadRes(conn, br, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"1"},
				"Transport": inTH.Write(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		var th headers.Transport
		err = th.Read(res.Header["Transport"])
		require.NoError(t, err)
		ssrcs[0] = th.SSRC

		inTH = &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{2, 3},
		}

		res, err = writeReqReadRes(conn, br, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=1"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"2"},
				"Transport": inTH.Write(),
				"Session":   res.Header["Session"],
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		th = headers.Transport{}
		err = th.Read(res.Header["Transport"])
		require.NoError(t, err)
		ssrcs[1] = th.SSRC

		res, err = writeReqReadRes(conn, br, base.Request{
			Method: base.Play,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"3"},
				"Session": res.Header["Session"],
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		var ri headers.RTPInfo
		err = ri.Read(res.Header["RTP-Info"])
		require.NoError(t, err)

		return &ri, ssrcs
	}

	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track, track})

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				go func() {
					time.Sleep(1 * time.Second)
					stream.WritePacketRTP(1, []byte{0x01, 0x02, 0x03, 0x04})
					stream.WritePacketRTP(0, []byte{0x05, 0x06, 0x07, 0x08})
				}()

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	buf, err := (&rtp.Packet{
		Header: rtp.Header{
			Version:        0x80,
			PayloadType:    96,
			SequenceNumber: 556,
			Timestamp:      984512368,
			SSRC:           96342362,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}).Marshal()
	require.NoError(t, err)
	stream.WritePacketRTP(0, buf)

	rtpInfo, ssrcs := getInfos()
	require.Equal(t, &headers.RTPInfo{
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   "/teststream/trackID=0",
			}).String(),
			SequenceNumber: func() *uint16 {
				v := uint16(556)
				return &v
			}(),
			Timestamp: (*rtpInfo)[0].Timestamp,
		},
	}, rtpInfo)
	require.Equal(t, []*uint32{
		func() *uint32 {
			v := uint32(96342362)
			return &v
		}(),
		nil,
	}, ssrcs)

	buf, err = (&rtp.Packet{
		Header: rtp.Header{
			Version:        0x80,
			PayloadType:    96,
			SequenceNumber: 87,
			Timestamp:      756436454,
			SSRC:           536474323,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}).Marshal()
	require.NoError(t, err)
	stream.WritePacketRTP(1, buf)

	rtpInfo, ssrcs = getInfos()
	require.Equal(t, &headers.RTPInfo{
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   "/teststream/trackID=0",
			}).String(),
			SequenceNumber: func() *uint16 {
				v := uint16(556)
				return &v
			}(),
			Timestamp: (*rtpInfo)[0].Timestamp,
		},
		&headers.RTPInfoEntry{
			URL: (&base.URL{
				Scheme: "rtsp",
				Host:   "localhost:8554",
				Path:   "/teststream/trackID=1",
			}).String(),
			SequenceNumber: func() *uint16 {
				v := uint16(87)
				return &v
			}(),
			Timestamp: (*rtpInfo)[1].Timestamp,
		},
	}, rtpInfo)
	require.Equal(t, []*uint32{
		func() *uint32 {
			v := uint32(96342362)
			return &v
		}(),
		func() *uint32 {
			v := uint32(536474323)
			return &v
		}(),
	}, ssrcs)
}

func TestServerReadErrorUDPSamePorts(t *testing.T) {
	track, err := NewTrackH264(96, &TrackConfigH264{
		[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04},
	})
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		UDPRTPAddress:  "127.0.0.1:8000",
		UDPRTCPAddress: "127.0.0.1:8001",
		RTSPAddress:    "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	func() {
		conn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, err := writeReqReadRes(conn, br, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"1"},
				"Transport": inTH.Write(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)

		res, err = writeReqReadRes(conn, br, base.Request{
			Method: base.Play,
			URL:    mustParseURL("rtsp://localhost:8554/teststream"),
			Header: base.Header{
				"CSeq":    base.HeaderValue{"2"},
				"Session": res.Header["Session"],
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusOK, res.StatusCode)
	}()

	func() {
		conn, err := net.Dial("tcp", "localhost:8554")
		require.NoError(t, err)
		defer conn.Close()
		br := bufio.NewReader(conn)

		inTH := &headers.Transport{
			Delivery: func() *headers.TransportDelivery {
				v := headers.TransportDeliveryUnicast
				return &v
			}(),
			Mode: func() *headers.TransportMode {
				v := headers.TransportModePlay
				return &v
			}(),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		}

		res, err := writeReqReadRes(conn, br, base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
			Header: base.Header{
				"CSeq":      base.HeaderValue{"1"},
				"Transport": inTH.Write(),
			},
		})
		require.NoError(t, err)
		require.Equal(t, base.StatusBadRequest, res.StatusCode)
	}()
}
