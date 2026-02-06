package rtpreceiver

import (
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestErrorInvalidPeriod(t *testing.T) {
	rr := &Receiver{
		ClockRate: 90000,
		LocalSSRC: 0x65f83afb,
	}
	err := rr.Initialize()
	require.EqualError(t, err, "invalid Period")
}

func TestErrorDifferentSSRC(t *testing.T) {
	rr := &Receiver{
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
	rr := &Receiver{
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

			rr := &Receiver{
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
			require.Equal(t, &Stats{
				RemoteSSRC:         0xba9da416,
				LastRTP:            0xafb45733,
				LastSequenceNumber: 945,
				TotalReceived:      1,
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
			require.Equal(t, &Stats{
				RemoteSSRC:         0xba9da416,
				LastRTP:            0xafb45733,
				LastSequenceNumber: 945,
				LastNTP:            time.Date(2008, 5, 20, 22, 15, 20, 0, time.UTC).Local(),
				TotalReceived:      1,
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
			require.Equal(t, &Stats{
				RemoteSSRC:         0xba9da416,
				LastRTP:            2947921603,
				LastSequenceNumber: 947,
				LastNTP:            time.Date(2008, 5, 20, 22, 15, 21, 0, time.UTC).Local(),
				TotalReceived:      3,
			}, stats)
		})
	}
}

func TestNoPackets(t *testing.T) {
	pktGenerated := make(chan rtcp.Packet)

	rr := &Receiver{
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

	rr := &Receiver{
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
	require.Equal(t, &Stats{
		RemoteSSRC:         0xba9da416,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 945,
		TotalReceived:      1,
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
	require.Equal(t, &Stats{
		RemoteSSRC:         0xba9da416,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 945,
		TotalReceived:      1,
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
	require.Equal(t, &Stats{
		RemoteSSRC:         0xba9da416,
		LastRTP:            2947921603,
		LastSequenceNumber: 947,
		TotalReceived:      3,
	}, stats)
}

func TestSequenceNumberOverflow(t *testing.T) {
	rtcpGenerated := make(chan struct{})

	rr := &Receiver{
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

	rr := &Receiver{
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

	rr := &Receiver{
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
	require.Equal(t, &Stats{
		RemoteSSRC:         0xba9da416,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 290,
		LastNTP:            time.Date(2020, 11, 21, 17, 44, 36, 869277776, time.UTC).Local(),
		TotalReceived:      2,
		TotalLost:          1,
	}, stats)
}

func TestReliableOverflowAndPacketsLost(t *testing.T) {
	done := make(chan struct{})

	rr := &Receiver{
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
	require.Equal(t, &Stats{
		RemoteSSRC:         0xba9da416,
		LastRTP:            0xafb45733,
		LastSequenceNumber: 2,
		LastNTP:            time.Date(2020, 11, 21, 17, 44, 36, 869277776, time.UTC).Local(),
		TotalReceived:      2,
		TotalLost:          2,
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
			// packet sent before first packet
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

	rr := &Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		Period:               500 * time.Millisecond,
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	rr.absPos = 40

	for _, entry := range sequence {
		var out []*rtp.Packet
		var missing uint64
		out, missing, err = rr.ProcessPacket(entry.in, time.Time{}, true)
		require.NoError(t, err)
		require.Equal(t, entry.out, out)
		require.Equal(t, uint64(0), missing)
	}

	stats := rr.Stats()
	require.Equal(t, &Stats{
		RemoteSSRC:         0,
		LastRTP:            0,
		LastSequenceNumber: 1,
		TotalReceived:      8,
	}, stats)
}

func TestUnrealiableBufferFull(t *testing.T) {
	rtcpReceived := make(chan struct{})

	rr := &Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		Period:               500 * time.Millisecond,
		WritePacketRTCP: func(p rtcp.Packet) {
			require.Equal(t, &rtcp.ReceiverReport{
				SSRC: 1710766843,
				Reports: []rtcp.ReceptionReport{{
					SSRC:               0,
					FractionLost:       131,
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

	rr.absPos = 25
	sn := uint16(1564)
	toMiss := uint64(34)

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
	sn++

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
	require.Equal(t, &Stats{
		RemoteSSRC:         0,
		LastRTP:            0,
		LastSequenceNumber: 1629,
		TotalReceived:      32,
		TotalLost:          34,
	}, stats)
}

func TestUnrealiableReset(t *testing.T) {
	rtcpGenerated := make(chan struct{})

	rr := &Receiver{
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
	require.Equal(t, &Stats{
		RemoteSSRC:         0,
		LastRTP:            0,
		LastSequenceNumber: 40064,
		TotalReceived:      2,
	}, stats)
}

func TestUnrealiableCustomBufferSize(t *testing.T) {
	customSize := 128

	rr := &Receiver{
		ClockRate:            90000,
		LocalSSRC:            0x65f83afb,
		UnrealiableTransport: true,
		BufferSize:           customSize,
		Period:               500 * time.Millisecond,
	}
	err := rr.Initialize()
	require.NoError(t, err)
	defer rr.Close()

	// Set absPos to an arbitrary value.
	rr.absPos = 10

	// Process first packet; behaves as usual.
	firstSeq := uint16(50)
	out, missing, err := rr.ProcessPacket(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: firstSeq,
		},
	}, time.Time{}, true)
	require.NoError(t, err)
	require.Equal(t, []*rtp.Packet{{
		Header: rtp.Header{
			SequenceNumber: firstSeq,
		},
	}}, out)
	require.Equal(t, uint64(0), missing)

	// At this point, expectedSeqNum == firstSeq + 1 (i.e. 51).
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
	require.Equal(t, &Stats{
		LastSequenceNumber: 181,
		TotalReceived:      2,
		TotalLost:          130,
	}, stats)
}
