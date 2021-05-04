package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

// This example shows how to
// 1. create a RTSP server which accepts plain connections
// 2. allow a single client to publish a stream with TCP or UDP
// 3. allow multiple clients to read that stream with TCP or UDP

type serverHandler struct {
	mutex     sync.Mutex
	publisher *gortsplib.ServerSession
	readers   map[*gortsplib.ServerSession]struct{}
	sdp       []byte
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(sc *gortsplib.ServerConn) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(sc *gortsplib.ServerConn, err error) {
	log.Printf("conn closed (%v)", err)
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(ss *gortsplib.ServerSession) {
	log.Printf("session opened")
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(ss *gortsplib.ServerSession) {
	log.Printf("session closed")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if ss == sh.publisher {
		sh.publisher = nil
		sh.sdp = nil
	} else {
		delete(sh.readers, ss)
	}
}

// called after receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, []byte, error) {
	log.Printf("describe request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// no one is publishing yet
	if sh.publisher == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.sdp, nil
}

// called after receiving an ANNOUNCE request.
func (sh *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	log.Printf("announce request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sh.publisher != nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("someone is already publishing")
	}

	sh.publisher = ctx.Session
	sh.sdp = ctx.Tracks.Write()

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, error) {
	log.Printf("setup request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("play request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	sh.readers[ctx.Session] = struct{}{}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a RECORD request.
func (sh *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	log.Printf("record request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if ctx.Session != sh.publisher {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("someone is already publishing")
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a frame.
func (sh *serverHandler) OnFrame(ctx *gortsplib.ServerHandlerOnFrameCtx) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// if we are the publisher, route frames to readers
	if ctx.Session == sh.publisher {
		for r := range sh.readers {
			r.WriteFrame(ctx.TrackID, ctx.StreamType, ctx.Payload)
		}
	}
}

func main() {
	// configure server
	s := &gortsplib.Server{
		Handler:        &serverHandler{},
		UDPRTPAddress:  ":8000",
		UDPRTCPAddress: ":8001",
	}

	// start server and wait until a fatal error
	log.Printf("server is ready")
	panic(s.StartAndWait(":8554"))
}
