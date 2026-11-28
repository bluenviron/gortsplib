package gortsplib

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
)

func TestClientPlaySenderReportBeforeRTP(t *testing.T) {
	for _, scenario := range []struct {
		protocol       Protocol
		matchingSource bool
	}{
		{ProtocolTCP, true},
		{ProtocolUDP, true},
		{ProtocolTCP, false},
		{ProtocolUDP, false},
	} {
		t.Run(fmt.Sprintf("%s/matching-source=%t", scenario.protocol, scenario.matchingSource), func(t *testing.T) {
			var stream *ServerStream
			var address string
			server := &Server{
				RTSPAddress:              "127.0.0.1:0",
				DisableRTCPSenderReports: true,
				Listen: func(network, requested string) (net.Listener, error) {
					listener, err := net.Listen(network, requested)
					if err == nil {
						address = listener.Addr().String()
					}
					return listener, err
				},
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
			if scenario.protocol == ProtocolUDP {
				server.UDPRTPAddress = "127.0.0.1:8000"
				server.UDPRTCPAddress = "127.0.0.1:8001"
			}
			require.NoError(t, server.Start())
			t.Cleanup(server.Close)
			stream = &ServerStream{Server: server, Desc: &description.Session{Medias: []*description.Media{testH264Media}}}
			require.NoError(t, stream.Initialize())
			t.Cleanup(stream.Close)
			url := mustParseURL("rtsp://" + address + "/stream")
			client := &Client{Scheme: url.Scheme, Host: url.Host, Protocol: new(scenario.protocol)}
			require.NoError(t, client.Start())
			t.Cleanup(client.Close)
			desc, _, err := client.Describe(url)
			require.NoError(t, err)
			require.NoError(t, client.SetupAll(desc.BaseURL, desc.Medias))
			reports := make(chan struct{})
			samples := make(chan startupClockSample)
			client.OnPacketRTCPAny(func(_ *description.Media, packet rtcp.Packet) {
				if _, ok := packet.(*rtcp.SenderReport); ok {
					reports <- struct{}{}
				}
			})
			client.OnPacketRTPAny(func(media *description.Media, _ format.Format, packet *rtp.Packet) {
				absolute, available := client.PacketNTP(media, packet)
				samples <- startupClockSample{available: available, absolute: absolute}
			})
			_, err = client.Play(nil)
			require.NoError(t, err)
			origin := time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)
			source := stream.Stats().Medias[testH264Media].Formats[testH264Media.Formats[0]].LocalSSRC
			if !scenario.matchingSource {
				source++
			}
			require.NoError(t, stream.WritePacketRTCP(testH264Media, &rtcp.SenderReport{
				SSRC: source, NTPTime: ntp.Encode(origin), RTPTime: 54352,
			}))
			<-reports
			require.NoError(t, stream.WritePacketRTPWithNTP(testH264Media, &rtp.Packet{
				Header:  rtp.Header{Version: 2, Marker: true, PayloadType: 96, SequenceNumber: 1, Timestamp: 54352},
				Payload: []byte{1, 2, 3, 4},
			}, origin))
			first := <-samples
			require.Equal(t, scenario.matchingSource, first.available)
			if scenario.matchingSource {
				require.Equal(t, origin, first.absolute.UTC())
			}
		})
	}
}
