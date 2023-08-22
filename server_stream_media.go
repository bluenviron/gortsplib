package gortsplib

import (
	"github.com/bluenviron/gortsplib/v4/pkg/description"
)

type serverStreamMedia struct {
	st              *ServerStream
	media           *description.Media
	trackID         int
	formats         map[uint8]*serverStreamFormat
	multicastWriter *serverMulticastWriter
}

func newServerStreamMedia(st *ServerStream, medi *description.Media, trackID int) *serverStreamMedia {
	sm := &serverStreamMedia{
		st:      st,
		media:   medi,
		trackID: trackID,
	}

	sm.formats = make(map[uint8]*serverStreamFormat)
	for _, forma := range medi.Formats {
		sm.formats[forma.PayloadType()] = newServerStreamFormat(
			sm,
			forma)
	}

	return sm
}

func (sm *serverStreamMedia) close() {
	for _, tr := range sm.formats {
		if tr.rtcpSender != nil {
			tr.rtcpSender.Close()
		}
	}

	if sm.multicastWriter != nil {
		sm.multicastWriter.close()
	}
}

func (sm *serverStreamMedia) allocateMulticastHandler(s *Server) error {
	if sm.multicastWriter == nil {
		mh, err := newServerMulticastWriter(s)
		if err != nil {
			return err
		}

		sm.multicastWriter = mh
	}
	return nil
}

func (sm *serverStreamMedia) writePacketRTCP(byts []byte) {
	// send unicast
	for r := range sm.st.activeUnicastReaders {
		sm, ok := r.setuppedMedias[sm.media]
		if ok {
			sm.writePacketRTCP(byts)
		}
	}

	// send multicast
	if sm.multicastWriter != nil {
		sm.multicastWriter.writePacketRTCP(byts)
	}
}
