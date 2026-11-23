package gortsplib

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// A camera that sends RTP with a payload type missing from the SDP must be counted
// separately (and still as an error), without touching the valid packets.
func TestClientPlayUnknownPayloadTypeStats(t *testing.T) {
	const unknownCount = 7
	const validCount = 5

	for _, transport := range []string{"udp", "tcp"} {
		t.Run(transport, func(t *testing.T) {
			var stream *ServerStream

			s := &Server{
				RTSPAddress: "127.0.0.1:" + strconv.Itoa(freeLoopbackPort(t)),
				Handler: &testServerHandler{
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{StatusCode: base.StatusOK}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{StatusCode: base.StatusOK}, stream, nil
					},
					onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{StatusCode: base.StatusOK}, nil
					},
				},
			}
			if transport == "udp" {
				rtpPort := freeLoopbackPort(t) &^ 1
				s.UDPRTPAddress = "127.0.0.1:" + strconv.Itoa(rtpPort)
				s.UDPRTCPAddress = "127.0.0.1:" + strconv.Itoa(rtpPort+1)
			}
			require.NoError(t, s.Start())
			defer s.Close()

			// The server declares PT 50 so it can send it; the client is given a
			// description without it, like a camera that sends an undeclared type.
			extra := &format.Generic{PayloadTyp: 50, RTPMa: "private/90000"}
			require.NoError(t, extra.Init())
			serverMedia := &description.Media{
				Type:    description.MediaTypeVideo,
				Formats: []format.Format{testH264Media.Formats[0], extra},
			}
			stream = &ServerStream{
				Server: s,
				Desc:   &description.Session{Medias: []*description.Media{serverMedia}},
			}
			require.NoError(t, stream.Initialize())
			defer stream.Close()

			proto := ProtocolUDP
			if transport == "tcp" {
				proto = ProtocolTCP
			}
			c := &Client{Scheme: "rtsp", Host: s.RTSPAddress, Protocol: &proto}
			require.NoError(t, c.Start())
			defer c.Close()

			u, err := base.ParseURL("rtsp://" + s.RTSPAddress + "/teststream")
			require.NoError(t, err)
			desc, _, err := c.Describe(u)
			require.NoError(t, err)
			require.Len(t, desc.Medias[0].Formats, 2)
			desc.Medias[0].Formats = desc.Medias[0].Formats[:1]
			require.NoError(t, c.SetupAll(desc.BaseURL, desc.Medias))

			valid := make(chan struct{}, validCount*2)
			c.OnPacketRTPAny(func(_ *description.Media, _ format.Format, _ *rtp.Packet) {
				valid <- struct{}{}
			})
			_, err = c.Play(nil)
			require.NoError(t, err)

			medi := stream.Desc.Medias[0]
			for i := 0; i < validCount; i++ {
				pkt := testRTPPacket
				pkt.SequenceNumber = uint16(i)
				require.NoError(t, stream.WritePacketRTP(medi, &pkt))
			}
			for i := 0; i < unknownCount; i++ {
				pkt := testRTPPacket
				pkt.PayloadType = 50
				pkt.SequenceNumber = uint16(100 + i)
				require.NoError(t, stream.WritePacketRTP(medi, &pkt))
			}

			require.Eventually(t, func() bool {
				return c.Stats().Session.InboundRTPPacketsInError >= unknownCount
			}, 5*time.Second, 20*time.Millisecond)
			require.Eventually(t, func() bool { return len(valid) >= validCount },
				5*time.Second, 20*time.Millisecond)

			st := c.Stats().Session
			require.Equal(t, uint64(unknownCount), st.InboundRTPPacketsUnknownPayloadType)
			require.Equal(t, uint64(unknownCount), st.InboundRTPPacketsInError)
			require.Equal(t, uint64(validCount), st.InboundRTPPackets)
			ms := st.Medias[desc.Medias[0]]
			require.Equal(t, uint64(unknownCount), ms.InboundRTPPacketsUnknownPayloadType)
		})
	}
}
