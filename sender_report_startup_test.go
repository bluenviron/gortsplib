package gortsplib

import (
	"bufio"
	"net"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
)

type startupClockSample struct {
	available bool
	absolute  time.Time
}

type startupClockPublisher struct {
	send     func(int, []byte) error
	reports  chan struct{}
	samples  chan startupClockSample
	invalid  chan error
	sequence uint16
}

func newStartupClockPublisher(t *testing.T, protocol Protocol) *startupClockPublisher {
	t.Helper()
	f := &startupClockPublisher{
		reports: make(chan struct{}),
		samples: make(chan startupClockSample),
		invalid: make(chan error),
	}
	var address string
	server := &Server{
		RTSPAddress: "127.0.0.1:0",
		Listen: func(network, requested string) (net.Listener, error) {
			listener, err := net.Listen(network, requested)
			if err == nil {
				address = listener.Addr().String()
			}
			return listener, err
		},
		Handler: &testServerHandler{
			onAnnounce: func(_ *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				return &base.Response{StatusCode: base.StatusOK}, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{StatusCode: base.StatusOK}, nil, nil
			},
			onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
				ctx.Session.OnPacketRTCPAny(func(_ *description.Media, _ rtcp.Packet) {
					f.reports <- struct{}{}
				})
				ctx.Session.OnPacketRTPAny(func(media *description.Media, _ format.Format, packet *rtp.Packet) {
					absolute, available := ctx.Session.PacketNTP(media, packet)
					f.samples <- startupClockSample{available: available, absolute: absolute}
				})
				return &base.Response{StatusCode: base.StatusOK}, nil
			},
			onDecodeError: func(ctx *ServerHandlerOnDecodeErrorCtx) { f.invalid <- ctx.Error },
		},
	}
	if protocol == ProtocolUDP {
		server.UDPRTPAddress = "127.0.0.1:8000"
		server.UDPRTCPAddress = "127.0.0.1:8001"
	}
	require.NoError(t, server.Start())
	t.Cleanup(server.Close)
	wire, err := net.Dial("tcp", address)
	require.NoError(t, err)
	t.Cleanup(func() { wire.Close() })
	connection := conn.NewConn(bufio.NewReader(wire), wire)
	medias := []*description.Media{{
		Type:    description.MediaTypeVideo,
		Control: "trackID=0",
		Formats: []format.Format{
			&format.H264{PayloadTyp: 96, PacketizationMode: 1},
			&format.H264{PayloadTyp: 97, PacketizationMode: 1},
		},
	}}
	url := "rtsp://" + address + "/stream"
	doAnnounce(t, connection, url, medias)
	transport := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       new(headers.TransportDeliveryUnicast),
		Mode:           new(headers.TransportModeRecord),
		InterleavedIDs: &[2]int{0, 1},
	}
	f.send = func(channel int, payload []byte) error {
		frame := &base.InterleavedFrame{Channel: channel, Payload: payload}
		return connection.WriteInterleavedFrame(frame, make([]byte, frame.MarshalSize()))
	}
	if protocol == ProtocolUDP {
		data, dataError := net.ListenPacket("udp", "127.0.0.1:0")
		require.NoError(t, dataError)
		t.Cleanup(func() { data.Close() })
		control, controlError := net.ListenPacket("udp", "127.0.0.1:0")
		require.NoError(t, controlError)
		t.Cleanup(func() { control.Close() })
		transport.Protocol = headers.TransportProtocolUDP
		transport.InterleavedIDs = nil
		transport.ClientPorts = &[2]int{data.LocalAddr().(*net.UDPAddr).Port, control.LocalAddr().(*net.UDPAddr).Port}
		response, selected := doSetup(t, connection, url+"/trackID=0", transport, "")
		doRecord(t, connection, url, readSession(t, response))
		f.send = func(channel int, payload []byte) error {
			socket := data
			if channel == 1 {
				socket = control
			}
			_, writeError := socket.WriteTo(payload, &net.UDPAddr{
				IP: net.ParseIP("127.0.0.1"), Port: selected.ServerPorts[channel],
			})
			return writeError
		}
	} else {
		response, _ := doSetup(t, connection, url+"/trackID=0", transport, "")
		doRecord(t, connection, url, readSession(t, response))
	}
	return f
}

func (f *startupClockPublisher) report(t *testing.T, source, timestamp uint32, absolute time.Time) {
	t.Helper()
	require.NoError(t, f.send(1, mustMarshalPacketRTCP(&rtcp.SenderReport{
		SSRC: source, RTPTime: timestamp, NTPTime: ntp.Encode(absolute),
	})))
	<-f.reports
}

func (f *startupClockPublisher) packet(t *testing.T, payloadType uint8, source, timestamp uint32) startupClockSample {
	t.Helper()
	f.sequence++
	require.NoError(t, f.send(0, mustMarshalPacketRTP(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    payloadType,
			SequenceNumber: f.sequence,
			Timestamp:      timestamp,
			SSRC:           source,
		},
		Payload: []byte{1, 2, 3, 4},
	})))
	return <-f.samples
}

func TestServerRecordSenderReportBeforeRTP(t *testing.T) {
	for _, protocol := range []Protocol{ProtocolTCP, ProtocolUDP} {
		t.Run(protocol.String(), func(t *testing.T) {
			f := newStartupClockPublisher(t, protocol)
			origin := time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)
			f.report(t, 753621, 54352, origin)
			first := f.packet(t, 96, 753621, 54352)
			require.True(t, first.available)
			require.Equal(t, origin, first.absolute.UTC())
		})
	}
}

func TestServerRecordEarlySenderReportsRemainSourceBound(t *testing.T) {
	f := newStartupClockPublisher(t, ProtocolTCP)
	firstOrigin := time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)
	secondOrigin := firstOrigin.Add(24 * time.Hour)
	f.report(t, 11, 10000, firstOrigin)
	f.report(t, 22, 20000, secondOrigin)
	second := f.packet(t, 97, 22, 20000)
	first := f.packet(t, 96, 11, 10000)
	require.True(t, second.available)
	require.Equal(t, secondOrigin, second.absolute.UTC())
	require.True(t, first.available)
	require.Equal(t, firstOrigin, first.absolute.UTC())
}

func TestServerRecordForeignReportCannotSupplyAbsoluteTime(t *testing.T) {
	f := newStartupClockPublisher(t, ProtocolTCP)
	origin := time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)
	f.report(t, 11, 10000, origin)
	first := f.packet(t, 96, 22, 10000)
	require.False(t, first.available)
	f.report(t, 22, 10000, origin.Add(time.Hour))
	next := f.packet(t, 96, 22, 10000)
	require.True(t, next.available)
	require.Equal(t, origin.Add(time.Hour), next.absolute.UTC())
}
