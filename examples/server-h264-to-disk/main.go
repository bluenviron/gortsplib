package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
)

// This example shows how to
// 1. create a RTSP server which accepts plain connections
// 2. allow a single client to publish a stream, containing a H264 media, with TCP or UDP
// 3. save the content of the H264 media in a file in MPEG-TS format

type serverHandler struct {
	s           *gortsplib.Server
	mutex       sync.Mutex
	publisher   *gortsplib.ServerSession
	media       *description.Media
	format      *format.H264
	rtpDec      *rtph264.Decoder
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

	sh.publisher = nil
	sh.mpegtsMuxer.close()
}

// called when receiving an ANNOUNCE request.
func (sh *serverHandler) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	log.Printf("announce request")

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sh.publisher != nil {
		sh.publisher.Close()
		sh.mpegtsMuxer.close()
	}

	// find the H264 media and format
	var forma *format.H264
	medi := ctx.Description.FindFormat(&forma)
	if medi == nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("H264 media not found")
	}

	// setup RTP -> H264 decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup H264 -> MPEGTS muxer
	mpegtsMuxer := &mpegtsMuxer{
		fileName: "mystream.ts",
		sps:      forma.SPS,
		pps:      forma.PPS,
	}
	err = mpegtsMuxer.initialize()
	if err != nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, err
	}

	sh.publisher = ctx.Session
	sh.media = medi
	sh.format = forma
	sh.rtpDec = rtpDec
	sh.mpegtsMuxer = mpegtsMuxer

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

// called when receiving a SETUP request.
func (sh *serverHandler) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("setup request")

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil, nil
}

// called when receiving a RECORD request.
func (sh *serverHandler) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	log.Printf("record request")

	// called when receiving a RTP packet
	ctx.Session.OnPacketRTP(sh.media, sh.format, func(pkt *rtp.Packet) {
		// decode timestamp
		pts, ok := ctx.Session.PacketPTS2(sh.media, pkt)
		if !ok {
			return
		}

		au, err := sh.rtpDec.Decode(pkt)
		if err != nil {
			return
		}

		// encode H264 access unit into MPEG-TS
		sh.mpegtsMuxer.writeH264(au, pts)
	})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func main() {
	// configure the server
	h := &serverHandler{}
	h.s = &gortsplib.Server{
		Handler:           h,
		RTSPAddress:       ":8554",
		UDPRTPAddress:     ":8000",
		UDPRTCPAddress:    ":8001",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}

	// start server and wait until a fatal error
	log.Printf("server is ready")
	panic(h.s.StartAndWait())
}
