package gortsplib

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

type serverStreamFormat struct {
	ssm       *serverStreamMedia
	format    format.Format
	localSSRC uint32

	multicastWriter    *serverMulticastWriterFormat
	mutex              sync.RWMutex
	firstSent          bool
	rtpPacketsSent     uint64
	lastSequenceNumber uint16
	lastRTP            uint32
	lastNTP            time.Time
}

func (ssf *serverStreamFormat) initialize() {
}

func (ssf *serverStreamFormat) rtpInfoEntry(now time.Time) *headers.RTPInfoEntry {
	clockRate := ssf.format.ClockRate()
	if clockRate == 0 {
		return nil
	}

	ssf.mutex.RLock()
	defer ssf.mutex.RUnlock()

	if !ssf.firstSent {
		return nil
	}

	// sequence number of the first packet of the stream
	seqNum := ssf.lastSequenceNumber + 1

	// RTP timestamp corresponding to the time value in
	// the Range response header.
	// remove a small quantity in order to avoid DTS > PTS
	ts := uint32(uint64(ssf.lastRTP) +
		uint64(now.Sub(ssf.lastNTP).Seconds()*float64(clockRate)) -
		uint64(clockRate)/10)

	return &headers.RTPInfoEntry{
		SequenceNumber: &seqNum,
		Timestamp:      &ts,
	}
}

func (ssf *serverStreamFormat) stats() ServerStreamStatsFormat {
	ssf.mutex.RLock()
	defer ssf.mutex.RUnlock()

	rtpPacketsSent := ssf.rtpPacketsSent

	return ServerStreamStatsFormat{
		OutboundRTPPackets: rtpPacketsSent,
		LocalSSRC:          ssf.localSSRC,
		RTPPacketsSent:     rtpPacketsSent,
	}
}

func (ssf *serverStreamFormat) writePacketRTP(pkt *rtp.Packet, ntp time.Time) error {
	pkt.SSRC = ssf.localSSRC

	maxPlainPacketSize := ssf.ssm.st.Server.MaxPacketSize
	if ssf.ssm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtpOverhead
	}

	plain := make([]byte, maxPlainPacketSize)
	n, err := pkt.MarshalTo(plain)
	if err != nil {
		return err
	}
	plain = plain[:n]

	var encr []byte
	if ssf.ssm.srtpOutCtx != nil {
		encr = make([]byte, ssf.ssm.st.Server.MaxPacketSize)
		encr, err = ssf.ssm.srtpOutCtx.encryptRTP(encr, plain, &pkt.Header)
		if err != nil {
			return err
		}
	}

	ptsEqualsDTS := ssf.format.PTSEqualsDTS(pkt)
	encrReaders := uint64(0)
	plainReaders := uint64(0)

	// send unicast
	for r := range ssf.ssm.st.activeUnicastReaders {
		if rsm, ok := r.setuppedMedias[ssf.ssm.media]; ok {
			rsf := rsm.formats[pkt.PayloadType]

			var buf []byte
			if rsm.srtpOutCtx != nil {
				buf = encr
				encrReaders++
			} else {
				buf = plain
				plainReaders++
			}
			err = rsf.writePacketRTPEncoded(pkt, ntp, ptsEqualsDTS, buf)
			if err != nil {
				r.onStreamWriteError(err)
				continue
			}
		}
	}

	// send multicast
	if ssf.ssm.multicastWriter != nil {
		var buf []byte
		if ssf.ssm.srtpOutCtx != nil {
			buf = encr
			encrReaders++
		} else {
			buf = plain
			plainReaders++
		}

		err = ssf.multicastWriter.writePacketRTPEncoded(pkt, ntp, ptsEqualsDTS, buf)
		if err != nil {
			return err
		}
	}

	ssf.ssm.bytesSent.Add(uint64(len(encr))*encrReaders + uint64(len(plain))*plainReaders)

	ssf.mutex.Lock()
	ssf.firstSent = true
	ssf.rtpPacketsSent += encrReaders + plainReaders
	ssf.lastSequenceNumber = pkt.SequenceNumber
	ssf.lastRTP = pkt.Timestamp
	ssf.lastNTP = ntp
	ssf.mutex.Unlock()

	return nil
}
