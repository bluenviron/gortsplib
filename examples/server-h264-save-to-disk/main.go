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
// 2. allow a single client to publish a stream, containing a H264 track, with TCP or UDP
// 3. save the content of the H264 track into a file in MPEG-TS format

type serverHandler struct {
	mutex       sync.Mutex
	publisher   *gortsplib.ServerSession
	h264TrackID int
	h264track   *gortsplib.TrackH264
	mpegtsMuxer *mpegtsMuxer
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

	// allow someone else to publish
	sh.publisher = nil
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

	// find the H264 track
	h264TrackID, h264track := func() (int, *gortsplib.TrackH264) {
		for i, track := range ctx.Tracks {
			if h264track, ok := track.(*gortsplib.TrackH264); ok {
				return i, h264track
			}
		}
		return -1, nil
	}()
	if h264TrackID < 0 {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("H264 track not found")
	}

	// setup H264->MPEGTS encoder
	mpegtsMuxer, err := newMPEGTSMuxer(h264track.SafeSPS(), h264track.SafePPS())
	if err != nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, err
	}

	sh.publisher = ctx.Session
	sh.h264TrackID = h264TrackID
	sh.mpegtsMuxer = mpegtsMuxer

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called after receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("setup request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil, nil
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

	if ctx.TrackID != sh.h264TrackID {
		return
	}

	if ctx.H264NALUs == nil {
		return
	}

	// encode H264 NALUs into MPEG-TS
	err := sh.mpegtsMuxer.encode(ctx.H264NALUs, ctx.H264PTS)
	if err != nil {
		return
	}
}

func main() {
	// configure server
	s := &gortsplib.Server{
		Handler:           &serverHandler{},
		RTSPAddress:       ":8554",
		UDPRTPAddress:     ":8000",
		UDPRTCPAddress:    ":8001",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}

	// start server and wait until a fatal error
	log.Printf("server is ready")
	panic(s.StartAndWait())
}
