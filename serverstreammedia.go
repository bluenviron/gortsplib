package gortsplib

type serverStreamMedia struct {
	formats          map[uint8]*serverStreamFormat
	multicastHandler *serverMulticastHandler
}

func (sm *serverStreamMedia) close() {
	for _, tr := range sm.formats {
		if tr.rtcpSender != nil {
			tr.rtcpSender.Close()
		}
	}

	if sm.multicastHandler != nil {
		sm.multicastHandler.close()
	}
}

func (sm *serverStreamMedia) allocateMulticastHandler(s *Server) error {
	if sm.multicastHandler == nil {
		mh, err := newServerMulticastHandler(s)
		if err != nil {
			return err
		}

		sm.multicastHandler = mh
	}
	return nil
}
