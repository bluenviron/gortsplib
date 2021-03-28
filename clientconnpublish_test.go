package gortsplib

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/rtcpreceiver"
)

func TestClientPublishSerial(t *testing.T) {
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
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
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
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				buf := make([]byte, 2048)
				err = req.ReadIgnoreFrames(bconn.Reader, buf)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

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

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			err = conn.WriteFrame(track.ID, StreamTypeRTP,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.NoError(t, err)

			conn.Close()

			err = conn.WriteFrame(track.ID, StreamTypeRTP,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.Error(t, err)
		})
	}
}

func TestClientPublishParallel(t *testing.T) {
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
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
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
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				buf := make([]byte, 2048)
				err = req.ReadIgnoreFrames(bconn.Reader, buf)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

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

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			writerDone := make(chan struct{})
			defer func() { <-writerDone }()

			conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer conn.Close()

			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := conn.WriteFrame(track.ID, StreamTypeRTP,
						[]byte{0x01, 0x02, 0x03, 0x04})
					if err != nil {
						return
					}
				}
			}()

			time.Sleep(1 * time.Second)
		})
	}
}

func TestClientPublishPauseSerial(t *testing.T) {
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
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
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
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				buf := make([]byte, 2048)
				err = req.ReadIgnoreFrames(bconn.Reader, buf)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				buf = make([]byte, 2048)
				err = req.ReadIgnoreFrames(bconn.Reader, buf)
				require.NoError(t, err)
				require.Equal(t, base.Teardown, req.Method)

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

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer conn.Close()

			err = conn.WriteFrame(track.ID, StreamTypeRTP,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.NoError(t, err)

			_, err = conn.Pause()
			require.NoError(t, err)

			err = conn.WriteFrame(track.ID, StreamTypeRTP,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.Error(t, err)

			_, err = conn.Record()
			require.NoError(t, err)

			err = conn.WriteFrame(track.ID, StreamTypeRTP,
				[]byte{0x01, 0x02, 0x03, 0x04})
			require.NoError(t, err)
		})
	}
}

func TestClientPublishPauseParallel(t *testing.T) {
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
							string(base.Announce),
							string(base.Setup),
							string(base.Record),
							string(base.Pause),
						}, ", ")},
					},
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = req.Read(bconn.Reader)
				require.NoError(t, err)
				require.Equal(t, base.Announce, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
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
				require.Equal(t, base.Record, req.Method)

				err = base.Response{
					StatusCode: base.StatusOK,
				}.Write(bconn.Writer)
				require.NoError(t, err)

				buf := make([]byte, 2048)
				err = req.ReadIgnoreFrames(bconn.Reader, buf)
				require.NoError(t, err)
				require.Equal(t, base.Pause, req.Method)

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

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			writerDone := make(chan struct{})
			go func() {
				defer close(writerDone)

				t := time.NewTicker(50 * time.Millisecond)
				defer t.Stop()

				for range t.C {
					err := conn.WriteFrame(track.ID, StreamTypeRTP,
						[]byte{0x01, 0x02, 0x03, 0x04})
					if err != nil {
						return
					}
				}
			}()

			time.Sleep(1 * time.Second)

			_, err = conn.Pause()
			require.NoError(t, err)
			<-writerDone

			conn.Close()
		})
	}
}

func TestClientPublishRTCP(t *testing.T) {
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
					string(base.Announce),
					string(base.Setup),
					string(base.Record),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Announce, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
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
		require.Equal(t, base.Record, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)
		require.NoError(t, err)

		rr := rtcpreceiver.New(nil, 90000)

		var f base.InterleavedFrame
		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, StreamTypeRTP, f.StreamType)
		rr.ProcessFrame(time.Now(), StreamTypeRTP, f.Payload)

		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, StreamTypeRTCP, f.StreamType)
		pkt, err := rtcp.Unmarshal(f.Payload)
		require.NoError(t, err)
		sr, ok := pkt[0].(*rtcp.SenderReport)
		require.True(t, ok)
		require.Equal(t, &rtcp.SenderReport{
			SSRC:        753621,
			NTPTime:     sr.NTPTime,
			RTPTime:     sr.RTPTime,
			PacketCount: 1,
			OctetCount:  4,
		}, sr)
		rr.ProcessFrame(time.Now(), StreamTypeRTCP, f.Payload)

		err = base.InterleavedFrame{
			TrackID:    0,
			StreamType: StreamTypeRTCP,
			Payload:    rr.Report(time.Now()),
		}.Write(bconn.Writer)
		require.NoError(t, err)

		f.Payload = make([]byte, 2048)
		err = f.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, StreamTypeRTP, f.StreamType)

		err = req.Read(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Teardown, req.Method)

		base.Response{
			StatusCode: base.StatusOK,
		}.Write(bconn.Writer)

		conn.Close()
	}()

	conf := ClientConf{
		StreamProtocol: func() *StreamProtocol {
			v := StreamProtocolTCP
			return &v
		}(),
		senderReportPeriod: 1 * time.Second,
	}

	track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
	require.NoError(t, err)

	conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
		Tracks{track})
	require.NoError(t, err)
	defer conn.Close()

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
	err = conn.WriteFrame(track.ID, StreamTypeRTP, byts)
	require.NoError(t, err)

	time.Sleep(1300 * time.Millisecond)

	err = conn.WriteFrame(track.ID, StreamTypeRTP, byts)
	require.NoError(t, err)
}
