package rtcpreceiver

import (
	"testing"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

func TestRtcpReceiver(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v)

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
	byts, _ := rtpPkt.Marshal()
	rr.OnFrame(base.StreamTypeRtp, byts)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	byts, _ = srPkt.Marshal()
	rr.OnFrame(base.StreamTypeRtcp, byts)

	res := rr.Report()

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: uint32(946),
				LastSenderReport:   uint32(0x887a17ce),
			},
		},
	}
	expected, _ := expectedPkt.Marshal()
	require.Equal(t, expected, res)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 945,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	byts, _ = rtpPkt.Marshal()
	rr.OnFrame(base.StreamTypeRtp, byts)

	res = rr.Report()

	expectedPkt = rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: uint32(1<<16 | 945),
				LastSenderReport:   uint32(0x887a17ce),
			},
		},
	}
	expected, _ = expectedPkt.Marshal()
	require.Equal(t, expected, res)
}
