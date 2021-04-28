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
	publisher *gortsplib.ServerConn
	readers   map[*gortsplib.ServerConn]struct{}
	sdp       []byte
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(sc *gortsplib.ServerConn) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(sc *gortsplib.ServerConn, err error) {
	log.Println("conn closed (%v)", err)

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sc == sh.publisher {
		sh.publisher = nil
		sh.sdp = nil
	} else {
		delete(sh.readers, sc)
	}
}

// called after receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, []byte, error) {
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
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sh.publisher != nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("someone is already publishing")
	}

	sh.publisher = ctx.Conn
	sh.sdp = ctx.Tracks.Write()

	return &base.Response{
		StatusCode: base.StatusOK,
		Header: base.Header{
			"Session": base.HeaderValue{"12345678"},
		},
	}, nil
}

// called after receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
		Header: base.Header{
			"Session": base.HeaderValue{"12345678"},
		},
	}, nil
}

// called after receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	sh.readers[ctx.Conn] = struct{}{}

	return &base.Response{
		StatusCode: base.StatusOK,
		Header: base.Header{
			"Session": base.HeaderValue{"12345678"},
		},
	}, nil
}

// called after receiving a RECORD request.
func (sh *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if ctx.Conn != sh.publisher {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("someone is already publishing")
	}

	return &base.Response{
		StatusCode: base.StatusOK,
		Header: base.Header{
			"Session": base.HeaderValue{"12345678"},
		},
	}, nil
}

// called after receiving a frame.
func (sh *serverHandler) OnFrame(ctx *gortsplib.ServerHandlerOnFrameCtx) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	// if we are the publisher, route frames to readers
	if ctx.Conn == sh.publisher {
		for r := range sh.readers {
			r.WriteFrame(ctx.TrackID, ctx.StreamType, ctx.Payload)
		}
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
		Handler: &serverHandler{},
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	// start server and wait until a fatal error
	panic(s.StartAndWait(":8554"))
}
