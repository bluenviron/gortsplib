package rtpsender

import (
	"sync"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestSender(t *testing.T) {
	var curTime time.Time
	var mutex sync.Mutex

	setCurTime := func(v time.Time) {
		mutex.Lock()
		defer mutex.Unlock()
		curTime = v
	}

	pktGenerated := make(chan rtcp.Packet)

	rs := &Sender{
		ClockRate: 90000,
		Period:    100 * time.Millisecond,
		TimeNow: func() time.Time {
			mutex.Lock()
			defer mutex.Unlock()
			return curTime
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			pktGenerated <- pkt
		},
	}
	rs.Initialize()
	defer rs.Close()

	stats := rs.Stats()
	require.Nil(t, stats)

	setCurTime(time.Date(2008, 5, 20, 22, 16, 20, 0, time.UTC))
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
	rs.ProcessPacket(&rtpPkt, ts, true)

	setCurTime(time.Date(2008, 5, 20, 22, 16, 22, 0, time.UTC))
	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 947,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 21, 0, time.UTC)
	rs.ProcessPacket(&rtpPkt, ts, true)

	rtpPkt = rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 948,
			Timestamp:      1287987768,
			SSRC:           0xba9da416,
		},
		Payload: []byte("\x00\x00"),
	}
	ts = time.Date(2008, 0o5, 20, 22, 15, 22, 0, time.UTC)
	rs.ProcessPacket(&rtpPkt, ts, false)

	setCurTime(time.Date(2008, 5, 20, 22, 16, 24, 0, time.UTC))

	pkt := <-pktGenerated
	require.Equal(t, &rtcp.SenderReport{
		SSRC: 0xba9da416,
		NTPTime: func() uint64 {
			d := time.Date(2008, 5, 20, 22, 15, 23, 0, time.UTC)
			s := uint64(d.UnixNano()) + 2208988800*1000000000
			return (s/1000000000)<<32 | (s % 1000000000)
		}(),
		RTPTime:     1287987768 + 2*90000,
		PacketCount: 3,
		OctetCount:  6,
	}, pkt)

	stats = rs.Stats()
	require.Equal(t, &Stats{
		LastSequenceNumber: 948,
		LastRTP:            1287987768,
		LastNTP:            time.Date(2008, time.May, 20, 22, 15, 21, 0, time.UTC),
	}, stats)
}

func TestSenderZeroClockRate(t *testing.T) {
	var curTime time.Time
	var mutex sync.Mutex

	setCurTime := func(v time.Time) {
		mutex.Lock()
		defer mutex.Unlock()
		curTime = v
	}

	pktGenerated := make(chan rtcp.Packet)

	rs := &Sender{
		ClockRate: 0,
		Period:    100 * time.Millisecond,
		TimeNow: func() time.Time {
			mutex.Lock()
			defer mutex.Unlock()
			return curTime
		},
		WritePacketRTCP: func(pkt rtcp.Packet) {
			pktGenerated <- pkt
		},
	}
	rs.Initialize()
	defer rs.Close()

	stats := rs.Stats()
	require.Nil(t, stats)

	setCurTime(time.Date(2008, 5, 20, 22, 16, 20, 0, time.UTC))
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
	rs.ProcessPacket(&rtpPkt, ts, true)

	select {
	case <-pktGenerated:
		t.Error("should not happen")
	case <-time.After(500 * time.Millisecond):
	}

	stats = rs.Stats()
	require.Equal(t, &Stats{
		LastSequenceNumber: 946,
		LastRTP:            1287987768,
		LastNTP:            time.Date(2008, time.May, 20, 22, 15, 20, 0, time.UTC),
	}, stats)
}
