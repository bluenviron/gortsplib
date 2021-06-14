package gortsplib

import (
	"net"
	"sync"

	"github.com/aler9/gortsplib/pkg/base"
)

type listenerPair struct {
	rtpListener  *serverUDPListener
	rtcpListener *serverUDPListener
}

// ServerStream is an entity that allows to send frames to multiple readers.
type ServerStream struct {
	s      *Server
	tracks Tracks

	mutex              sync.RWMutex
	readersUnicast     map[*ServerSession]struct{}
	readers            map[*ServerSession]struct{}
	multicastListeners []*listenerPair
}

// NewServerStream allocates a ServerStream.
func NewServerStream(tracks Tracks) *ServerStream {
	return &ServerStream{
		tracks:         tracks,
		readersUnicast: make(map[*ServerSession]struct{}),
		readers:        make(map[*ServerSession]struct{}),
	}
}

// Close closes a ServerStream.
func (st *ServerStream) Close() error {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if st.s != nil {
		select {
		case st.s.streamRemove <- st:
		case <-st.s.ctx.Done():
		}
	}

	if st.multicastListeners != nil {
		for _, l := range st.multicastListeners {
			l.rtpListener.close()
			l.rtcpListener.close()
		}
		st.multicastListeners = nil
	}

	for ss := range st.readers {
		ss.Close()
	}
	st.readers = nil

	return nil
}

// Tracks returns the tracks of the stream.
func (st *ServerStream) Tracks() Tracks {
	return st.tracks
}

func (st *ServerStream) readerAdd(ss *ServerSession, isMulticast bool) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	st.readers[ss] = struct{}{}

	if !isMulticast {
		return
	}

	if st.s == nil {
		st.s = ss.s
		select {
		case st.s.streamAdd <- st:
		case <-st.s.ctx.Done():
		}
	}

	if st.multicastListeners != nil {
		return
	}

	st.multicastListeners = make([]*listenerPair, len(st.tracks))

	for i := range st.tracks {
		rtpListener, rtcpListener := newServerUDPListenerMulticastPair(st.s)
		st.multicastListeners[i] = &listenerPair{
			rtpListener:  rtpListener,
			rtcpListener: rtcpListener,
		}
	}
}

func (st *ServerStream) readerRemove(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	delete(st.readers, ss)

	if len(st.readers) == 0 && st.multicastListeners != nil {
		for _, l := range st.multicastListeners {
			l.rtpListener.close()
			l.rtcpListener.close()
		}
	}
}

func (st *ServerStream) readerSetActive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if *ss.setuppedDelivery == base.StreamDeliveryUnicast {
		st.readersUnicast[ss] = struct{}{}
	}
}

func (st *ServerStream) readerSetInactive(ss *ServerSession) {
	st.mutex.Lock()
	defer st.mutex.Unlock()

	if *ss.setuppedDelivery == base.StreamDeliveryUnicast {
		delete(st.readersUnicast, ss)
	}
}

// WriteFrame writes a frame to all the readers of the stream.
func (st *ServerStream) WriteFrame(trackID int, streamType StreamType, payload []byte) {
	st.mutex.RLock()
	defer st.mutex.RUnlock()

	// send unicast
	for r := range st.readersUnicast {
		r.WriteFrame(trackID, streamType, payload)
	}

	// send multicast
	if st.multicastListeners != nil {
		if streamType == StreamTypeRTP {
			st.multicastListeners[trackID].rtpListener.write(payload, &net.UDPAddr{
				IP:   multicastIP,
				Zone: "",
				Port: st.multicastListeners[trackID].rtpListener.port(),
			})
		} else {
			st.multicastListeners[trackID].rtcpListener.write(payload, &net.UDPAddr{
				IP:   multicastIP,
				Zone: "",
				Port: st.multicastListeners[trackID].rtcpListener.port(),
			})
		}
	}
}
