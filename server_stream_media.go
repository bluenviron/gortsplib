package gortsplib

import (
	"fmt"
	"sync/atomic"

	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/pion/rtcp"
)

type serverStreamMedia struct {
	st         *ServerStream
	media      *description.Media
	trackID    int
	localSSRCs map[uint8]uint32
	srtpOutCtx *wrappedSRTPContext

	formats         map[uint8]*serverStreamFormat
	multicastWriter *serverMulticastWriter
	bytesSent       *uint64
	rtcpPacketsSent *uint64
}

func (sm *serverStreamMedia) initialize() {
	sm.bytesSent = new(uint64)
	sm.rtcpPacketsSent = new(uint64)

	sm.formats = make(map[uint8]*serverStreamFormat)

	for _, forma := range sm.media.Formats {
		sf := &serverStreamFormat{
			sm:        sm,
			format:    forma,
			localSSRC: sm.localSSRCs[forma.PayloadType()],
		}
		sf.initialize()
		sm.formats[forma.PayloadType()] = sf
	}
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
	plain, err := pkt.Marshal()
	if err != nil {
		return err
	}

	maxPlainPacketSize := sm.st.Server.MaxPacketSize
	if sm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtcpOverhead
	}

	if len(plain) > maxPlainPacketSize {
		return fmt.Errorf("packet is too big")
	}

	var encr []byte
	if sm.srtpOutCtx != nil {
		encr = make([]byte, sm.st.Server.MaxPacketSize)
		encr, err = sm.srtpOutCtx.encryptRTCP(encr, plain, nil)
		if err != nil {
			return err
		}
	}

	encrLen := uint64(len(encr))
	plainLen := uint64(len(plain))

	// send unicast
	for r := range sm.st.activeUnicastReaders {
		if sm, ok := r.setuppedMedias[sm.media]; ok {
			if isSecure(r.setuppedTransport.Profile) {
				err = sm.writePacketRTCPEncoded(encr)
				if err != nil {
					r.onStreamWriteError(err)
					continue
				}

				atomic.AddUint64(sm.bytesSent, encrLen)
			} else {
				err = sm.writePacketRTCPEncoded(plain)
				if err != nil {
					r.onStreamWriteError(err)
					continue
				}

				atomic.AddUint64(sm.bytesSent, plainLen)
			}

			atomic.AddUint64(sm.rtcpPacketsSent, 1)
		}
	}

	// send multicast
	if sm.multicastWriter != nil {
		if sm.srtpOutCtx != nil {
			err = sm.multicastWriter.writePacketRTCP(encr)
			if err != nil {
				return err
			}

			atomic.AddUint64(sm.bytesSent, encrLen)
		} else {
			err = sm.multicastWriter.writePacketRTCP(plain)
			if err != nil {
				return err
			}

			atomic.AddUint64(sm.bytesSent, plainLen)
		}

		atomic.AddUint64(sm.rtcpPacketsSent, 1)
	}

	return nil
}
