package main

import (
	"log"
	"net"
	"sync"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

type server struct {
	s      *gortsplib.Server
	mutex  sync.Mutex
	stream *gortsplib.ServerStream
}

func (s *server) initialize() {
	// configure the server
	s.s = &gortsplib.Server{
		Handler:           s,
		RTSPAddress:       ":8554",
		UDPRTPAddress:     ":8000",
		UDPRTCPAddress:    ":8001",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}
}

// called when a connection is opened.
func (s *server) OnConnOpen(ctx *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (s *server) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("conn closed (%v)", ctx.Error)
}

// called when a session is opened.
func (s *server) OnSessionOpen(ctx *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("session opened")
}

// called when a session is closed.
func (s *server) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("session closed")
}

// called when receiving a DESCRIBE request.
func (s *server) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("describe request")

	// Check for backchannel
	requestBackchannel := false
	for _, value := range ctx.Request.Header["Require"] {
		if value == "www.onvif.org/ver20/backchannel" {
			requestBackchannel = true
		}
	}

	if requestBackchannel {
		log.Printf("backchanel requested")
	}

	// HACK!!!
	// Can we add in the extra Media description only when backchannel is requested?

	s.mutex.Lock()
	defer s.mutex.Unlock()

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
func (s *server) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("setup request")

	s.mutex.Lock()
	defer s.mutex.Unlock()

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
func (s *server) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("play request")

	// assign the OnPacketRTPAny callback function so we can receive the backchannel audio
	ctx.Session.OnPacketRTPAny(func(medi *description.Media, format format.Format, pkt *rtp.Packet) {
		log.Printf("Got %s RTP data from VMS at %d size %d", format.Codec(), pkt.Timestamp, len(pkt.Payload))

		// Marshal the RTP Packet back into bytes and send via UDP
		bytes, err := pkt.Marshal()
		if err != nil {
			return
		}

		// Send the RTP Payload from Port 9002 to destination port 127.0.0.1:9004 (note port numbers must be Even for RTP)
		// Play the audio with
		//     gst-launch-1.0 udpsrc port=9004 caps=application/x-rtp ! queue ! rtppcmudepay ! autoaudiosink sync=false
		// or with
		//     ffplay  -reorder_queue_size 0 rtp://0.0.0.0:9004          (use reorder_queue_size because if RTP packets stop then resume, FFMPEG complains RTP packets are too late)
		//
		conn, err := net.ListenPacket("udp", ":9002") // TODO - keep this open
		if err != nil {
			return
		}

		defer conn.Close()

		dst, err := net.ResolveUDPAddr("udp", "127.0.0.1:9004")
		if err != nil {
			return
		}

		_, err = conn.WriteTo(bytes, dst)
		if err != nil {
			return
		}
	})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (s *server) setStreamReady(desc *description.Session) *gortsplib.ServerStream {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.stream = gortsplib.NewServerStream(s.s, desc)
	return s.stream
}

func (s *server) setStreamUnready() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.stream.Close()
	s.stream = nil
}
