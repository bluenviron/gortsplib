package rtcpsender

import (
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestRTCPSender(t *testing.T) {
	now = func() time.Time {
		return time.Date(2008, 5, 20, 22, 16, 20, 600000000, time.UTC)
	}
	done := make(chan struct{})

	rs := New(500*time.Millisecond, 90000, func(pkt rtcp.Packet) {
		require.Equal(t, &rtcp.SenderReport{
			SSRC:        0xba9da416,
			NTPTime:     0xcbddcc34999997ff,
			RTPTime:     0x4d185ae8,
			PacketCount: 3,
			OctetCount:  6,
		}, pkt)
		close(done)
	})
	defer rs.Close()

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rs.ProcessPacketRTP(ts, &rtpPkt, true)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 947,
			Timestamp:      1287987768 + 45000,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 500000000, time.UTC)
	rs.ProcessPacketRTP(ts, &rtpPkt, true)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 948,
			Timestamp:      1287987768 + 90000,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 500000000, time.UTC)
	rs.ProcessPacketRTP(ts, &rtpPkt, false)

	<-done
}
