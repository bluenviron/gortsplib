// Package main contains an example.
package main

import (
	"log"
	"sync"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

// This example shows how to:
// 1. create a RTSP server which accepts plain connections.
// 2. create a stream with an audio direct channel and an audio back channel.
// 3. write the audio direct channel to readers, read the back channel from readers.

type serverHandler struct {
	server *gortsplib.Server
	stream *gortsplib.ServerStream
	mutex  sync.RWMutex
}

// called when a connection is opened.
func (sh *serverHandler) OnConnOpen(_ *gortsplib.ServerHandlerOnConnOpenCtx) {
	log.Printf("conn opened")
}

// called when a connection is closed.
func (sh *serverHandler) OnConnClose(ctx *gortsplib.ServerHandlerOnConnCloseCtx) {
	log.Printf("conn closed (%v)", ctx.Error)
}

// called when a session is opened.
func (sh *serverHandler) OnSessionOpen(_ *gortsplib.ServerHandlerOnSessionOpenCtx) {
	log.Printf("session opened")
}

// called when a session is closed.
func (sh *serverHandler) OnSessionClose(_ *gortsplib.ServerHandlerOnSessionCloseCtx) {
	log.Printf("session closed")
}

// called when receiving a DESCRIBE request.
func (sh *serverHandler) OnDescribe(
	_ *gortsplib.ServerHandlerOnDescribeCtx,
) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("DESCRIBE request")

	sh.mutex.RLock()
	defer sh.mutex.RUnlock()

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving a SETUP request.
func (sh *serverHandler) OnSetup(
	_ *gortsplib.ServerHandlerOnSetupCtx,
) (*base.Response, *gortsplib.ServerStream, error) {
	log.Printf("SETUP request")

	sh.mutex.RLock()
	defer sh.mutex.RUnlock()

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

// called when receiving a PLAY request.
func (sh *serverHandler) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	log.Printf("PLAY request")

	// called when receiving a RTP packet
	ctx.Session.OnPacketRTPAny(func(m *description.Media, _ format.Format, pkt *rtp.Packet) {
		// decode timestamp
		pts, ok := ctx.Session.PacketPTS(m, pkt)
		if !ok {
			return
		}

		log.Printf("incoming RTP packet with PTS=%v size=%v", pts, len(pkt.Payload))
	})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func main() {
	h := &serverHandler{}

	// prevent clients from connecting to the server until the stream is properly set up
	h.mutex.Lock()

	// create the server
	h.server = &gortsplib.Server{
		Handler:           h,
		RTSPAddress:       ":8554",
		UDPRTPAddress:     ":8000",
		UDPRTCPAddress:    ":8001",
		MulticastIPRange:  "224.1.0.0/16",
		MulticastRTPPort:  8002,
		MulticastRTCPPort: 8003,
	}

	// start the server
	err := h.server.Start()
	if err != nil {
		panic(err)
	}
	defer h.server.Close()

	// create a RTSP description
	desc := &description.Session{
		Medias: []*description.Media{
			// direct channel
			{
				Type: description.MediaTypeAudio,
				Formats: []format.Format{&format.G711{
					PayloadTyp:   8,
					MULaw:        false,
					SampleRate:   8000,
					ChannelCount: 1,
				}},
			},
			// back channel
			{
				Type:          description.MediaTypeAudio,
				IsBackChannel: true,
				Formats: []format.Format{&format.G711{
					PayloadTyp:   8,
					MULaw:        false,
					SampleRate:   8000,
					ChannelCount: 1,
				}},
			},
		},
	}

	// create a server stream
	h.stream = &gortsplib.ServerStream{
		Server: h.server,
		Desc:   desc,
	}
	err = h.stream.Initialize()
	if err != nil {
		panic(err)
	}
	defer h.stream.Close()

	// create audio streamer
	r := &audioStreamer{stream: h.stream}
	r.initialize()
	defer r.close()

	// allow clients to connect
	h.mutex.Unlock()

	// wait until a fatal error
	log.Printf("server is ready on %s", h.server.RTSPAddress)
	panic(h.server.Wait())
}
