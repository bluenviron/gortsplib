package gortsplib

import (
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestServerRecordEarlyReportReplacementAndTimestampWrap(t *testing.T) {
	f := newStartupClockPublisher(t, ProtocolTCP)
	origin := time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)
	baseTimestamp := uint32(0xfffff000)
	f.report(t, 0, baseTimestamp, origin.Add(-time.Hour))
	f.report(t, 0, baseTimestamp, origin)
	first := f.packet(t, 96, 0, baseTimestamp+90000)
	require.True(t, first.available)
	require.Equal(t, origin.Add(time.Second), first.absolute.UTC())
}

func TestServerRecordRetiredReportCannotSupplyAbsoluteTime(t *testing.T) {
	f := newStartupClockPublisher(t, ProtocolTCP)
	origin := time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)
	f.report(t, 11, 10000, origin)
	require.NoError(t, f.send(1, mustMarshalPacketRTCP(&rtcp.Goodbye{Sources: []uint32{11}})))
	<-f.reports
	first := f.packet(t, 96, 11, 10000)
	require.False(t, first.available)
}

func TestServerRecordMalformedRTPDoesNotBindAnEarlyClock(t *testing.T) {
	f := newStartupClockPublisher(t, ProtocolTCP)
	origin := time.Date(2018, 2, 20, 19, 0, 0, 0, time.UTC)
	f.report(t, 11, 10000, origin)
	malformed := mustMarshalPacketRTP(&rtp.Packet{
		Header:  rtp.Header{Version: 2, PayloadType: 96, SSRC: 22},
		Payload: []byte{255},
	})
	malformed[0] |= 0x20
	require.NoError(t, f.send(0, malformed))
	require.Error(t, <-f.invalid)
	first := f.packet(t, 96, 11, 10000)
	require.True(t, first.available)
	require.Equal(t, origin, first.absolute.UTC())
}
