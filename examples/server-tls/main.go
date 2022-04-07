package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

// This example shows how to
// 1. create a RTSP server which accepts only connections encrypted with TLS (RTSPS)
// 2. allow a single client to publish a stream with TCP
// 3. allow multiple clients to read that stream with TCP

type serverHandler struct {
	mutex     sync.Mutex
	stream    *gortsplib.ServerStream
	publisher *gortsplib.ServerSession
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("conn closed (%v)", ctx.Error)
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("session opened")
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("session closed")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// close stream-related listeners and disconnect every reader
	if sh.stream != nil && ctx.Session == sh.publisher {
		sh.stream.Close()
		sh.stream = nil
	}
}

// called after receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("describe request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called after receiving an ANNOUNCE request.
func (sh *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	log.Printf("announce request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sh.stream != nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("someone is already publishing")
	}

	sh.stream = gortsplib.NewServerStream(ctx.Tracks)
	sh.publisher = ctx.Session

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("setup request")

	// no one is publishing yet
	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called after receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("play request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a RECORD request.
func (sh *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	log.Printf("record request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a RTP packet.
func (sh *serverHandler) OnPacketRTP(ctx *gortsplib.ServerHandlerOnPacketRTPCtx) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// if we are the publisher, route the RTP packet to readers
	if ctx.Session == sh.publisher {
		sh.stream.WritePacketRTP(ctx.TrackID, ctx.Packet, ctx.PTSEqualsDTS)
	}
}

func main() {
	// setup certificates - they can be generated with
	// openssl genrsa -out server.key 2048
	// openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650
	cert, err := tls.LoadX509KeyPair("server.crt", "server.key")
	if err != nil {
		panic(err)
	}

	// configure server
	s := &gortsplib.Server{
		Handler:     &serverHandler{},
		TLSConfig:   &tls.Config{Certificates: []tls.Certificate{cert}},
		RTSPAddress: ":8554",
	}

	// start server and wait until a fatal error
	log.Printf("server is ready")
	panic(s.StartAndWait())
}
