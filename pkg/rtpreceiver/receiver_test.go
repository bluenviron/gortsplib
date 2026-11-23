package rtpreceiver_test

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpreceiver"
)

func TestErrorInvalidPeriod(t *testing.T) {
	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
	}
	err := rr.Initialize()
	require.EqualError(t, err, "invalid Period")
}

func TestErrorDifferentSSRC(t *testing.T) {
	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
		Period:    500 * time.Millisecond,
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 945,
			Timestamp:      0xafb45733,
			SSRC:           1434523,
		},
		Payload: []byte("\x00\x00"),
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 945,
			Timestamp:      0xafb45733,
			SSRC:           754623214,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)
}

func TestStatsBeforeData(t *testing.T) {
	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
		Period:    500 * time.Millisecond,
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	stats := rr.Stats()
	require.Nil(t, stats)
}

func TestStandard(t *testing.T) {
	for _, ca := range []string{
		"reliable",
		"unrealiable",
	} {
		t.Run(ca, func(t *testing.T) {
			pktGenerated := make(chan rtcp.Packet)

			rr := &rtpreceiver.Receiver{
				ClockRate:            90000,
				LocalSSRC:            0x65f83afb,
				UnrealiableTransport: ca == "unrealiable",
				Period:               500 * time.Millisecond,
				TimeNow: func() time.Time {
					return time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
				},
				WritePacketRTCP: func(pkt rtcp.Packet) {
					pktGenerated <- pkt
				},
			}
			err := rr.Initialize()
			require.NoError(t, err)
			defer rr.Close()

			rtpPkt := rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 945,
					Timestamp:      0xafb45733,
					SSRC:           0xba9da416,
				},
				Payload: []byte("\x00\x00"),
			}
			ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
			_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
			require.NoError(t, err)

			stats := rr.Stats()
			require.Equal(t, &rtpreceiver.Stats{
				Received:           1,
				LastRTP:            0xafb45733,
				LastSequenceNumber: 945,
				TotalReceived:      1,
				RemoteSSRC:         0xba9da416,
			}, stats)

			srPkt := rtcp.SenderReport{
				SSRC: 0xba9da416,
				NTPTime: func() uint64 {
					d := time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC)
					s := uint64(d.UnixNano()) + 2208988800*1000000000
					return (s/1000000000)<<32 | (s % 1000000000)
				}(),
				RTPTime:     0xafb45733,
				PacketCount: 714,
				OctetCount:  859127,
			}
			ts = time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC)
			rr.ProcessSenderReport(&srPkt, ts)

			stats = rr.Stats()
			require.Equal(t, &rtpreceiver.Stats{
				Received:           1,
				LastRTP:            0xafb45733,
				LastSequenceNumber: 945,
				LastNTP:            time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC).Local(),
				TotalReceived:      1,
				RemoteSSRC:         0xba9da416,
			}, stats)

			rtpPkt = rtp.Packet{
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
			ts = time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC)
			_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
			require.NoError(t, err)

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
			_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
			require.NoError(t, err)

			pkt := <-pktGenerated
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 947,
						LastSenderReport:   3422027776,
						Delay:              2 * 65536,
					},
				},
			}, pkt)

			stats = rr.Stats()
			require.Equal(t, &rtpreceiver.Stats{
				Received:           3,
				LastRTP:            2947921603,
				LastSequenceNumber: 947,
				LastNTP:            time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC).Local(),
				TotalReceived:      3,
				RemoteSSRC:         0xba9da416,
			}, stats)
		})
	}
}

func TestNoPackets(t *testing.T) {
	pktGenerated := make(chan rtcp.Packet)

	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			pktGenerated <- pkt
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 945,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-pktGenerated

	// do not send any packet here

	<-pktGenerated
}

func TestZeroClockRate(t *testing.T) {
	pktGenerated := make(chan rtcp.Packet)

	rr := &rtpreceiver.Receiver{
		ClockRate: 0,
		LocalSSRC: 0x65f83afb,
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			pktGenerated <- pkt
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	stats := rr.Stats()
	require.Nil(t, stats)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 945,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	stats = rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           1,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 945,
		TotalReceived:      1,
		RemoteSSRC:         0xba9da416,
	}, stats)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     0xafb45733,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

	stats = rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           1,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 945,
		TotalReceived:      1,
		RemoteSSRC:         0xba9da416,
	}, stats)

	rtpPkt = rtp.Packet{
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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	select {
	case <-pktGenerated:
		t.Error("should not happen")
	case <-time.After(500 * time.Millisecond):
	}

	stats = rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           3,
		LastRTP:            2947921603,
		LastSequenceNumber: 947,
		TotalReceived:      3,
		RemoteSSRC:         0xba9da416,
	}, stats)
}

func TestSequenceNumberOverflow(t *testing.T) {
	rtcpGenerated := make(chan struct{})

	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
		Period:    250 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 1 << 16,
						LastSenderReport:   0x887a17ce,
						Delay:              1 * 65536,
					},
				},
			}, pkt)
			close(rtcpGenerated)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	time.Sleep(400 * time.Millisecond)

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-rtcpGenerated
}

func TestJitter(t *testing.T) {
	rtcpGenerated := make(chan struct{})

	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 948,
						LastSenderReport:   0x887a17ce,
						Delay:              2 * 65536,
						Jitter:             45000 / 16,
					},
				},
			}, pkt)
			close(rtcpGenerated)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     0xafb45733,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 948,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, false)
	require.NoError(t, err)

	<-rtcpGenerated
}

func TestReliablePacketsLost(t *testing.T) {
	done := make(chan struct{})

	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
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
			}, pkt)
			close(done)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

	rtpPkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 288,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 290,
			Timestamp:      0xafb45733,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-done

	stats := rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           2,
		Lost:               1,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 290,
		LastNTP:            time.Date(2020, 11, 21, 17, 44, 36, 869277776, time.UTC).Local(),
		TotalReceived:      2,
		TotalLost:          1,
		RemoteSSRC:         0xba9da416,
	}, stats)
}

func TestReliableOverflowAndPacketsLost(t *testing.T) {
	done := make(chan struct{})

	rr := &rtpreceiver.Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
		Period:    500 * time.Millisecond,
		TimeNow: func() time.Time {
			return time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
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
			}, pkt)
			close(done)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	srPkt := rtcp.SenderReport{
		SSRC:        0xba9da416,
		NTPTime:     0xe363887a17ced916,
		RTPTime:     1287981738,
		PacketCount: 714,
		OctetCount:  859127,
	}
	ts := time.Date(2008, 0o5, 20, 22, 15, 20, 0, time.UTC)
	rr.ProcessSenderReport(&srPkt, ts)

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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

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
	_, _, err = rr.ProcessPacket(&rtpPkt, ts, true)
	require.NoError(t, err)

	<-done

	stats := rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           2,
		Lost:               2,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 2,
		LastNTP:            time.Date(2020, 11, 21, 17, 44, 36, 869277776, time.UTC).Local(),
		TotalReceived:      2,
		TotalLost:          2,
		RemoteSSRC:         stats.RemoteSSRC,
	}, stats)
}

func TestUnrealiableReorder(t *testing.T) {
	sequence := []struct {
		in  *rtp.Packet
		out []*rtp.Packet
	}{
		{
			// first packet
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65530,
				},
			},
			[]*rtp.Packet{{
				Header: rtp.Header{
					SequenceNumber: 65530,
				},
			}},
		},
		{
			// late duplicate
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65529,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// ok
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65531,
				},
			},
			[]*rtp.Packet{{
				Header: rtp.Header{
					SequenceNumber: 65531,
				},
			}},
		},
		{
			// duplicated
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65531,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// gap
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65535,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65533,
					PayloadType:    96,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered + duplicated
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65533,
					PayloadType:    97,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65532,
				},
			},
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						SequenceNumber: 65532,
					},
				},
				{
					Header: rtp.Header{
						SequenceNumber: 65533,
						PayloadType:    96,
					},
				},
			},
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65534,
				},
			},
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						SequenceNumber: 65534,
					},
				},
				{
					Header: rtp.Header{
						SequenceNumber: 65535,
					},
				},
			},
		},
		{
			// overflow + gap
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 1,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 0,
				},
			},
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						SequenceNumber: 0,
					},
				},
				{
					Header: rtp.Header{
						SequenceNumber: 1,
					},
				},
			},
		},
	}

	rr := &rtpreceiver.Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		Period:               500 * time.Millisecond,
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	for sequenceNumber := uint16(65490); sequenceNumber < 65530; sequenceNumber++ {
		var out []*rtp.Packet
		var missing uint64
		out, missing, err = rr.ProcessPacket(&rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sequenceNumber,
			},
		}, time.Time{}, true)
		require.NoError(t, err)
		require.Equal(t, []*rtp.Packet{{
			Header: rtp.Header{
				SequenceNumber: sequenceNumber,
			},
		}}, out)
		require.Equal(t, uint64(0), missing)
	}

	for _, entry := range sequence {
		var out []*rtp.Packet
		var missing uint64
		out, missing, err = rr.ProcessPacket(entry.in, time.Time{}, true)
		require.NoError(t, err)
		require.Equal(t, entry.out, out)
		require.Equal(t, uint64(0), missing)
	}

	stats := rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           48,
		LastRTP:            0,
		LastSequenceNumber: 1,
		TotalReceived:      48,
	}, stats)
}

func TestUnrealiableBufferFull(t *testing.T) {
	rtcpReceived := make(chan struct{})

	rr := &rtpreceiver.Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		Period:               500 * time.Millisecond,
		WritePacketRTCP: func(p rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 1710766843,
				Reports: []rtcp.ReceptionReport{{
					SSRC:               0,
					FractionLost:       95,
					TotalLost:          34,
					LastSequenceNumber: 1629,
					Jitter:             0,
					LastSenderReport:   0,
					Delay:              0,
				}},
			}, p)
			close(rtcpReceived)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	sn := uint16(1539)
	toMiss := uint64(34)

	var out []*rtp.Packet
	var missing uint64

	for range 26 {
		out, missing, err = rr.ProcessPacket(&rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sn,
			},
		}, time.Time{}, true)
		require.NoError(t, err)
		require.Equal(t, []*rtp.Packet{{
			Header: rtp.Header{
				SequenceNumber: sn,
			},
		}}, out)
		require.Equal(t, uint64(0), missing)
		sn++
	}

	var expected []*rtp.Packet

	for i := uint64(0); i < 64-toMiss; i++ {
		out, missing, err = rr.ProcessPacket(&rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sn + uint16(toMiss),
			},
		}, time.Time{}, true)
		require.NoError(t, err)
		require.Equal(t, []*rtp.Packet(nil), out)
		require.Equal(t, uint64(0), missing)

		expected = append(expected, &rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sn + uint16(toMiss),
			},
		})
		sn++
	}

	out, missing, err = rr.ProcessPacket(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn + uint16(toMiss),
		},
	}, time.Time{}, true)
	require.NoError(t, err)

	require.Equal(t, toMiss, missing)
	expected = append(expected, &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn + uint16(toMiss),
		},
	})
	require.Equal(t, expected, out)

	<-rtcpReceived

	stats := rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           57,
		Lost:               34,
		LastRTP:            0,
		LastSequenceNumber: 1629,
		TotalReceived:      57,
		TotalLost:          34,
	}, stats)
}

func TestUnrealiableReset(t *testing.T) {
	rtcpGenerated := make(chan struct{})

	rr := &rtpreceiver.Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		Period:               500 * time.Millisecond,
		WritePacketRTCP: func(p rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 1710766843,
				Reports: []rtcp.ReceptionReport{{
					SSRC:               0,
					LastSequenceNumber: 0x10000 + 40064,
				}},
			}, p)
			close(rtcpGenerated)
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	_, _, err = rr.ProcessPacket(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 50000,
		},
	}, time.Time{}, true)
	require.NoError(t, err)

	sn := uint16(40000)

	for range 64 {
		var out []*rtp.Packet
		var missing uint64
		out, missing, err = rr.ProcessPacket(&rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sn,
			},
		}, time.Time{}, true)
		require.NoError(t, err)
		require.Equal(t, []*rtp.Packet(nil), out)
		require.Equal(t, uint64(0), missing)
		sn++
	}

	out, missing, err := rr.ProcessPacket(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn,
		},
	}, time.Time{}, true)
	require.NoError(t, err)
	require.Equal(t, []*rtp.Packet{{
		Header: rtp.Header{
			SequenceNumber: sn,
		},
	}}, out)
	require.Equal(t, uint64(0), missing)

	<-rtcpGenerated

	stats := rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           2,
		LastRTP:            0,
		LastSequenceNumber: 40064,
		TotalReceived:      2,
	}, stats)
}

func TestUnrealiableCustomBufferSize(t *testing.T) {
	customSize := 128

	rr := &rtpreceiver.Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		BufferSize:           customSize,
		Period:               500 * time.Millisecond,
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	var out []*rtp.Packet
	var missing uint64

	for sequenceNumber := uint16(40); sequenceNumber <= 50; sequenceNumber++ {
		out, missing, err = rr.ProcessPacket(&rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sequenceNumber,
			},
		}, time.Time{}, true)
		require.NoError(t, err)
		require.Equal(t, []*rtp.Packet{{
			Header: rtp.Header{
				SequenceNumber: sequenceNumber,
			},
		}}, out)
		require.Equal(t, uint64(0), missing)
	}

	// At this point, expectedSeqNum == 51.
	// Now, send a packet with a gap larger than the custom buffer size.
	// For BufferSize = 128, let's send a packet with SequenceNumber = 51 + 130 = 181.
	nextSeq := uint16(181)
	out, missing, err = rr.ProcessPacket(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: nextSeq,
		},
	}, time.Time{}, true)
	require.NoError(t, err)

	// Since there are no packets buffered, n remains 1.
	// relPos = 181 - 51 = 130; so missing should be 130
	require.Equal(t, uint64(130), missing)
	require.Equal(t, []*rtp.Packet{{
		Header: rtp.Header{
			SequenceNumber: nextSeq,
		},
	}}, out)

	stats := rr.Stats()
	require.Equal(t, &rtpreceiver.Stats{
		Received:           12,
		Lost:               130,
		LastSequenceNumber: 181,
		TotalReceived:      12,
		TotalLost:          130,
	}, stats)
}

func ntp64Value(t time.Time) []byte {
	return binary.BigEndian.AppendUint64(nil, ntp.Encode(t))
}

// marshal and unmarshal the packet in order to test parsing of the extension.
func testNTP64Packet(t *testing.T, seq uint16, ts uint32, profile uint16, id uint8, ext []byte) *rtp.Packet {
	t.Helper()

	pkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: seq,
			Timestamp:      ts,
			SSRC:           0xba9da416,
		},
		Payload: []byte{1, 2},
	}

	if ext != nil {
		pkt.Extension = true
		pkt.ExtensionProfile = profile
		err := pkt.SetExtension(id, ext)
		require.NoError(t, err)
	}

	byts, err := pkt.Marshal()
	require.NoError(t, err)

	var out rtp.Packet
	err = out.Unmarshal(byts)
	require.NoError(t, err)

	return &out
}

func TestNTP64Extension(t *testing.T) {
	for _, ca := range []struct {
		name    string
		profile uint16
		id      uint8
	}{
		{
			"one-byte header",
			rtp.ExtensionProfileOneByte,
			3,
		},
		{
			"two-byte header",
			rtp.ExtensionProfileTwoByte,
			200,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			pktGenerated := make(chan rtcp.Packet)

			rr := &rtpreceiver.Receiver{
				ClockRate:        90000,
				LocalSSRC:        0x65f83afb,
				Period:           500 * time.Millisecond,
				NTP64ExtensionID: ca.id,
				TimeNow: func() time.Time {
					return time.Date(2008, 5, 20, 22, 15, 22, 0, time.UTC)
				},
				WritePacketRTCP: func(pkt rtcp.Packet) {
					pktGenerated <- pkt
				},
			}
			err := rr.Initialize()
			require.NoError(t, err)
			defer rr.Close()

			_, ok := rr.PacketNTP(0xafb45733)
			require.False(t, ok)

			pkts, _ := rr.ProcessPacket2(
				testNTP64Packet(t, 945, 0xafb45733, ca.profile, ca.id,
					ntp64Value(time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC))),
				time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC),
				true)
			require.Len(t, pkts, 1)

			v, ok := rr.PacketNTP(0xafb45733)
			require.True(t, ok)
			require.Equal(t, time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC), v.UTC())

			stats := rr.Stats()
			require.Equal(t, time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC), stats.LastNTP.UTC())

			// packets without the extension keep the mapping
			rr.ProcessPacket2(
				testNTP64Packet(t, 946, 0xafb45733+90000, 0, 0, nil),
				time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC),
				true)

			v, ok = rr.PacketNTP(0xafb45733 + 90000)
			require.True(t, ok)
			require.Equal(t, time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC), v.UTC())

			stats = rr.Stats()
			require.Equal(t, time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC), stats.LastNTP.UTC())

			// the extension is not a sender report
			pkt := <-pktGenerated
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 0x65f83afb,
				Reports: []rtcp.ReceptionReport{
					{
						SSRC:               0xba9da416,
						LastSequenceNumber: 946,
					},
				},
			}, pkt)
		})
	}
}

func TestNTP64ExtensionIgnored(t *testing.T) {
	for _, ca := range []struct {
		name    string
		id      uint8
		profile uint16
		extID   uint8
		ext     []byte
	}{
		{
			"disabled",
			0,
			rtp.ExtensionProfileOneByte,
			1,
			ntp64Value(time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC)),
		},
		{
			"different id",
			1,
			rtp.ExtensionProfileOneByte,
			2,
			ntp64Value(time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC)),
		},
		{
			"wrong length",
			1,
			rtp.ExtensionProfileOneByte,
			1,
			ntp64Value(time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC))[:4],
		},
		{
			"zero",
			1,
			rtp.ExtensionProfileOneByte,
			1,
			make([]byte, 8),
		},
		{
			"non-RFC 8285 profile",
			1,
			0xABAC,
			0,
			// would be ID 1 with 8 bytes in a one-byte header
			append(append([]byte{0x17}, ntp64Value(time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC))...), 0, 0, 0),
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			rr := &rtpreceiver.Receiver{
				ClockRate:        90000,
				LocalSSRC:        0x65f83afb,
				Period:           500 * time.Millisecond,
				NTP64ExtensionID: ca.id,
				WritePacketRTCP:  func(rtcp.Packet) {},
			}
			err := rr.Initialize()
			require.NoError(t, err)
			defer rr.Close()

			rr.ProcessPacket2(
				testNTP64Packet(t, 945, 0xafb45733, ca.profile, ca.extID, ca.ext),
				time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC),
				true)

			_, ok := rr.PacketNTP(0xafb45733)
			require.False(t, ok)

			stats := rr.Stats()
			require.Equal(t, time.Time{}, stats.LastNTP)
		})
	}
}

func TestNTP64ExtensionAndSenderReport(t *testing.T) {
	pktGenerated := make(chan rtcp.Packet)

	rr := &rtpreceiver.Receiver{
		ClockRate:        90000,
		LocalSSRC:        0x65f83afb,
		Period:           500 * time.Millisecond,
		NTP64ExtensionID: 1,
		TimeNow: func() time.Time {
			return time.Date(2008, 5, 20, 22, 15, 22, 0, time.UTC)
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			pktGenerated <- pkt
		},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	rr.ProcessPacket2(
		testNTP64Packet(t, 945, 0xafb45733, rtp.ExtensionProfileOneByte, 1,
			ntp64Value(time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC))),
		time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC),
		true)

	v, ok := rr.PacketNTP(0xafb45733)
	require.True(t, ok)
	require.Equal(t, time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC), v.UTC())

	// sender reports override the extension
	srNTP := ntp.Encode(time.Date(2008, 5, 20, 22, 15, 30, 0, time.UTC))
	rr.ProcessSenderReport(&rtcp.SenderReport{
		SSRC:    0xba9da416,
		NTPTime: srNTP,
		RTPTime: 0xafb45733,
	}, time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC))

	v, ok = rr.PacketNTP(0xafb45733)
	require.True(t, ok)
	require.Equal(t, time.Date(2008, 5, 20, 22, 15, 30, 0, time.UTC), v.UTC())

	// once a sender report has been received, the extension is ignored
	rr.ProcessPacket2(
		testNTP64Packet(t, 946, 0xafb45733+90000, rtp.ExtensionProfileOneByte, 1,
			ntp64Value(time.Date(2008, 5, 20, 22, 15, 40, 0, time.UTC))),
		time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC),
		true)

	v, ok = rr.PacketNTP(0xafb45733 + 90000)
	require.True(t, ok)
	require.Equal(t, time.Date(2008, 5, 20, 22, 15, 31, 0, time.UTC), v.UTC())

	pkt := <-pktGenerated
	require.Equal(t, &rtcp.ReceiverReport{
		SSRC: 0x65f83afb,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               0xba9da416,
				LastSequenceNumber: 946,
				LastSenderReport:   uint32(srNTP >> 16),
				Delay:              65536,
			},
		},
	}, pkt)
}

func TestNTP64ExtensionRefresh(t *testing.T) {
	rr := &rtpreceiver.Receiver{
		ClockRate:        90000,
		LocalSSRC:        0x65f83afb,
		Period:           500 * time.Millisecond,
		NTP64ExtensionID: 1,
		WritePacketRTCP:  func(rtcp.Packet) {},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	rr.ProcessPacket2(
		testNTP64Packet(t, 945, 0, rtp.ExtensionProfileOneByte, 1,
			ntp64Value(time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC))),
		time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC),
		true)

	// more than 2^31 ticks after the first extension
	rr.ProcessPacket2(
		testNTP64Packet(t, 946, 30000*90000, rtp.ExtensionProfileOneByte, 1,
			ntp64Value(time.Date(2008, 5, 21, 6, 35, 20, 0, time.UTC))),
		time.Date(2008, 5, 21, 6, 35, 20, 0, time.UTC),
		true)

	v, ok := rr.PacketNTP(30001 * 90000)
	require.True(t, ok)
	require.Equal(t, time.Date(2008, 5, 21, 6, 35, 21, 0, time.UTC), v.UTC())
}

func TestNTP64ExtensionReorder(t *testing.T) {
	rr := &rtpreceiver.Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		Period:               500 * time.Millisecond,
		NTP64ExtensionID:     1,
		WritePacketRTCP:      func(rtcp.Packet) {},
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	pkts, _ := rr.ProcessPacket2(
		testNTP64Packet(t, 945, 0xafb45733, 0, 0, nil),
		time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC),
		true)
	require.Len(t, pkts, 1)

	// buffered packets are not used until they are returned
	pkts, _ = rr.ProcessPacket2(
		testNTP64Packet(t, 947, 0xafb45733+2*3000, rtp.ExtensionProfileOneByte, 1,
			ntp64Value(time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC))),
		time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC),
		true)
	require.Len(t, pkts, 0)

	_, ok := rr.PacketNTP(0xafb45733 + 2*3000)
	require.False(t, ok)

	pkts, _ = rr.ProcessPacket2(
		testNTP64Packet(t, 946, 0xafb45733+3000, 0, 0, nil),
		time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC),
		true)
	require.Len(t, pkts, 2)

	v, ok := rr.PacketNTP(0xafb45733 + 2*3000)
	require.True(t, ok)
	require.Equal(t, time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC), v.UTC())
}
