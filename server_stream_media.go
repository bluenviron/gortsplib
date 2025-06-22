package gortsplib

import (
	"sync/atomic"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/pion/rtcp"
)

type serverStreamMedia struct {
	st      *ServerStream
	media   *description.Media
	trackID int

	formats         map[uint8]*serverStreamFormat
	multicastWriter *serverMulticastWriter
	bytesSent       *uint64
	rtcpPacketsSent *uint64
}

func (sm *serverStreamMedia) initialize() error {
	sm.bytesSent = new(uint64)
	sm.rtcpPacketsSent = new(uint64)

	sm.formats = make(map[uint8]*serverStreamFormat)

	for i, forma := range sm.media.Formats {
		sf := &serverStreamFormat{
			sm:     sm,
			format: forma,
		}
		err := sf.initialize()
		if err != nil {
			for _, forma := range sm.media.Formats[:i] {
				sm.formats[forma.PayloadType()].close()
			}
			return err
		}

		sm.formats[forma.PayloadType()] = sf
	}

	return nil
}

func (sm *serverStreamMedia) close() {
	for _, sf := range sm.formats {
		sf.close()
	}

	if sm.multicastWriter != nil {
		sm.multicastWriter.close()
	}
}

func (sm *serverStreamMedia) writePacketRTCP(pkt rtcp.Packet) error {
	byts, err := pkt.Marshal()
	if err != nil {
		return err
	}

	le := len(byts)

	// send unicast
	for r := range sm.st.activeUnicastReaders {
		if _, ok := r.setuppedMedias[sm.media]; ok {
			err := r.writePacketRTCPEncoded(sm.media, byts)
			if err != nil {
				r.onStreamWriteError(err)
				continue
			}

			atomic.AddUint64(sm.bytesSent, uint64(le))
			atomic.AddUint64(sm.rtcpPacketsSent, 1)
		}
	}

	// send multicast
	if sm.multicastWriter != nil {
		err := sm.multicastWriter.writePacketRTCP(byts)
		if err != nil {
			return err
		}

		atomic.AddUint64(sm.bytesSent, uint64(le))
		atomic.AddUint64(sm.rtcpPacketsSent, 1)
	}

	return nil
}
