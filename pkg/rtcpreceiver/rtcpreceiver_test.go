package rtcpreceiver

import (
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp/v2"
	"github.com/stretchr/testify/require"
)

func TestRTCPReceiverBase(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v, 90000)

	require.Equal(t, nil, rr.Report(time.Now()))

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     0xafb45733,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTCP(ts, &srPkt)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 947,
			Timestamp:      0xafb45733 + 90000,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 947,
				LastSenderReport:   0x887a17ce,
				Delay:              2 * 65536,
			},
		},
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
	require.Equal(t, &expectedPkt, rr.Report(ts))
}

func TestRTCPReceiverOverflow(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v, 90000)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTCP(ts, &srPkt)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0xffff,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0000,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

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
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, &expectedPkt, rr.Report(ts))
}

func TestRTCPReceiverPacketLost(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v, 90000)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTCP(ts, &srPkt)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0120,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0122,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

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
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, &expectedPkt, rr.Report(ts))
}

func TestRTCPReceiverOverflowPacketLost(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v, 90000)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTCP(ts, &srPkt)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0xffff,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x0002,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

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
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, &expectedPkt, rr.Report(ts))
}

func TestRTCPReceiverReorderedPackets(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v, 90000)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTCP(ts, &srPkt)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x43a7,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 0x43a6,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 0x43a7,
				LastSenderReport:   0x887a17ce,
				Delay:              1 * 65536,
			},
		},
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	require.Equal(t, &expectedPkt, rr.Report(ts))
}

func TestRTCPReceiverJitter(t *testing.T) {
	v := uint32(0x65f83afb)
	rr := New(&v, 90000)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     0xafb45733,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTCP(ts, &srPkt)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 946,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 947,
			Timestamp:      0xafb45733 + 45000,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	rr.ProcessPacketRTP(ts, &rtpPkt)

	expectedPkt := rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 947,
				LastSenderReport:   0x887a17ce,
				Delay:              2 * 65536,
				Jitter:             45000 / 16,
			},
		},
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
	require.Equal(t, &expectedPkt, rr.Report(ts))
}
