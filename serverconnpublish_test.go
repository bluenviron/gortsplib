package gortsplib

import (
	"bufio"
	"io"
	"net"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

func TestServerConnPublishReceivePackets(t *testing.T) {
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

				onAnnounce := func(req *base.Request, tracks Tracks) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onSetup := func(req *base.Request, th *headers.Transport, trackID int) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				onRecord := func(req *base.Request) (*base.Response, error) {
					return &base.Response{
						StatusCode: base.StatusOK,
					}, nil
				}

				rtpReceived := uint64(0)
				onFrame := func(trackID int, typ StreamType, buf []byte) {
					if atomic.SwapUint64(&rtpReceived, 1) == 0 {
						require.Equal(t, 0, trackID)
						require.Equal(t, StreamTypeRTP, typ)
						require.Equal(t, []byte("\x01\x02\x03\x04"), buf)
					} else {
						require.Equal(t, 0, trackID)
						require.Equal(t, StreamTypeRTCP, typ)
						require.Equal(t, []byte("\x05\x06\x07\x08"), buf)
						close(packetsReceived)
					}
				}

				err = <-conn.Read(ServerConnReadHandlers{
					OnAnnounce: onAnnounce,
					OnSetup:    onSetup,
					OnRecord:   onRecord,
					OnFrame:    onFrame,
				})
				require.Equal(t, io.EOF, err)
			}()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
			require.NoError(t, err)

			tracks := Tracks{track}

			for i, t := range tracks {
				t.ID = i
				t.BaseURL = base.MustParseURL("rtsp://localhost:8554/teststream")
				t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
					Key:   "control",
					Value: "trackID=" + strconv.FormatInt(int64(i), 10),
				})
			}

			err = base.Request{
				Method: base.Announce,
				URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: tracks.Write(),
			}.Write(bconn.Writer)
			require.NoError(t, err)

			var res base.Response
			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			th := &headers.Transport{
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModeRecord
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
					"CSeq":      base.HeaderValue{"2"},
					"Transport": th.Write(),
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			th, err = headers.ReadTransport(res.Header["Transport"])
			require.NoError(t, err)

			err = base.Request{
				Method: base.Record,
				URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq": base.HeaderValue{"3"},
				},
			}.Write(bconn.Writer)
			require.NoError(t, err)

			err = res.Read(bconn.Reader)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			if proto == "udp" {
				time.Sleep(1 * time.Second)

				l1, err := net.ListenPacket("udp", "localhost:35466")
				require.NoError(t, err)
				defer l1.Close()

				l1.WriteTo([]byte("\x01\x02\x03\x04"), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[0],
				})

				time.Sleep(100 * time.Millisecond)

				l1, err = net.ListenPacket("udp", "localhost:35467")
				require.NoError(t, err)
				defer l1.Close()

				l1.WriteTo([]byte("\x05\x06\x07\x08"), &net.UDPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: th.ServerPorts[1],
				})
			} else {
				err = base.InterleavedFrame{
					TrackID:    0,
					StreamType: StreamTypeRTP,
					Payload:    []byte("\x01\x02\x03\x04"),
				}.Write(bconn.Writer)
				require.NoError(t, err)

				err = base.InterleavedFrame{
					TrackID:    0,
					StreamType: StreamTypeRTCP,
					Payload:    []byte("\x05\x06\x07\x08"),
				}.Write(bconn.Writer)
				require.NoError(t, err)
			}

			<-packetsReceived
		})
	}
}
