package main

import (
	"log"
	"sync"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
)

type server struct {
	server *gortsplib.Server
	mutex  sync.RWMutex
	stream *gortsplib.ServerStream
}

func (s *server) initialize() {
	// configure the server
	s.server = &gortsplib.Server{
		Handler:           s,
		RTSPAddress:       ":8556",
		UDPRTPAddress:     ":8002",
		UDPRTCPAddress:    ":8003",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}
}

// called when a connection is opened.
func (s *server) OnConnOpen(_ *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (s *server) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("conn closed (%v)", ctx.Error)
}

// called when a session is opened.
func (s *server) OnSessionOpen(_ *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("session opened")
}

// called when a session is closed.
func (s *server) OnSessionClose(_ *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("session closed")
}

// called when receiving a DESCRIBE request.
func (s *server) OnDescribe(_ *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("DESCRIBE request")

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// stream is not available yet
	if s.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, s.stream, nil
}

// called when receiving a SETUP request.
func (s *server) OnSetup(_ *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("SETUP request")

	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// stream is not available yet
	if s.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, s.stream, nil
}

// called when receiving a PLAY request.
func (s *server) OnPlay(_ *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("PLAY request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (s *server) setStreamReady(desc *description.Session) *gortsplib.ServerStream {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.stream = &gortsplib.ServerStream{
		Server: s.server,
		Desc:   desc,
	}

	err := s.stream.Initialize()
	if err != nil {
		panic(err)
	}

	return s.stream
}

func (s *server) setStreamUnready() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.stream.Close()
	s.stream = nil
}
