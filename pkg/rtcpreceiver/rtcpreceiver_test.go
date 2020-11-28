package rtcpreceiver

import (
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

func TestRtcpReceiverBase(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	byts, _ := srPkt.Marshal()
	ts := time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtcp, byts)

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
	byts, _ = rtpPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtp, byts)

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 946,
				LastSenderReport:   0x887a17ce,
				Delay:              1 * 65536,
			},
		},
	}
	expected, _ := expectedPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, expected, rr.Report(ts))
}

func TestRtcpReceiverSequenceOverflow(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	byts, _ := srPkt.Marshal()
	ts := time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtcp, byts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0xffff,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	byts, _ = rtpPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtp, byts)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0000,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	byts, _ = rtpPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtp, byts)

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 1<<16 | 0x0000,
				LastSenderReport:   0x887a17ce,
				Delay:              1 * 65536,
			},
		},
	}
	expected, _ := expectedPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, expected, rr.Report(ts))
}

func TestRtcpReceiverPacketLost(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	byts, _ := srPkt.Marshal()
	ts := time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtcp, byts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0120,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	byts, _ = rtpPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtp, byts)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0122,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	byts, _ = rtpPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtp, byts)

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 0x0122,
				LastSenderReport:   0x887a17ce,
				FractionLost: func() uint8 {
					v := float64(1) / 3
					return uint8(v * 256)
				}(),
				TotalLost: 1,
				Delay:     1 * 65536,
			},
		},
	}
	expected, _ := expectedPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, expected, rr.Report(ts))
}

func TestRtcpReceiverSequenceOverflowPacketLost(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	byts, _ := srPkt.Marshal()
	ts := time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtcp, byts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0xffff,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	byts, _ = rtpPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtp, byts)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0002,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	byts, _ = rtpPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 20, 0, time.UTC)
	rr.OnFrame(ts, base.StreamTypeRtp, byts)

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 1<<16 | 0x0002,
				LastSenderReport:   0x887a17ce,
				FractionLost: func() uint8 {
					v := float64(2) / 4
					return uint8(v * 256)
				}(),
				TotalLost: 2,
				Delay:     1 * 65536,
			},
		},
	}
	expected, _ := expectedPkt.Marshal()
	ts = time.Date(2008, 05, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, expected, rr.Report(ts))
}
