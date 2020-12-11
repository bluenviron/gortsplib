// +build ignore

package main

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// This example shows how to
// 1. create a RTSP server
// 2. allow a single client to publish a stream with TCP
// 3. allow multiple clients to read that stream with TCP

var mutex sync.Mutex
var publisher *gortsplib.ServerConn
var sdp []byte
var readers = make(map[*gortsplib.ServerConn]struct{})

// this is called for each incoming connection
func handleConn(conn *gortsplib.ServerConn) {
	defer conn.Close()

	log.Printf("client connected")

	// this is called when a request arrives
	onRequest := func(req *base.Request) (*base.Response, error) {
		switch req.Method {
		// the Options method must return all available methods
		case base.Options:
			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Public": base.HeaderValue{strings.Join([]string{
						string(base.Describe),
						string(base.Announce),
						string(base.Setup),
						string(base.Play),
						string(base.Record),
						string(base.Teardown),
					}, ", ")},
				},
			}, nil

		// the Describe method must return the SDP of the stream
		case base.Describe:
			mutex.Lock()
			defer mutex.Unlock()

			// no one is publishing yet
			if publisher == nil {
				return &base.Response{
					StatusCode: base.StatusNotFound,
				}, nil
			}

			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Base": base.HeaderValue{req.URL.String() + "/"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Content: sdp,
			}, nil

		// the Announce method is called by publishers
		case base.Announce:
			ct, ok := req.Header["Content-Type"]
			if !ok || len(ct) != 1 {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("Content-Type header missing")
			}

			if ct[0] != "application/sdp" {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("unsupported Content-Type '%s'", ct)
			}

			tracks, err := gortsplib.ReadTracks(req.Content)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("invalid SDP: %s", err)
			}

			if len(tracks) == 0 {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("no tracks defined")
			}

			mutex.Lock()
			defer mutex.Unlock()

			if publisher != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("someone is already publishing")
			}

			publisher = conn
			sdp = tracks.Write()

			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Session": base.HeaderValue{"12345678"},
				},
			}, nil

		// The Setup method is called
		// * by publishers, after Announce
		// * by readers
		case base.Setup:
			th, err := headers.ReadTransport(req.Header["Transport"])
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("transport header: %s", err)
			}

			// support TCP only
			if th.Protocol == gortsplib.StreamProtocolUDP {
				return &base.Response{
					StatusCode: base.StatusUnsupportedTransport,
				}, nil
			}

			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Transport": req.Header["Transport"],
					"Session":   base.HeaderValue{"12345678"},
				},
			}, nil

		// The Play method is called by readers, after Setup
		case base.Play:
			mutex.Lock()
			defer mutex.Unlock()

			readers[conn] = struct{}{}

			conn.EnableReadFrames(true)
			conn.EnableReadTimeout(false)

			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Session": base.HeaderValue{"12345678"},
				},
			}, nil

		// The Record method is called by publishers, after Announce and Setup
		case base.Record:
			mutex.Lock()
			defer mutex.Unlock()

			if conn != publisher {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("someone is already publishing")
			}

			conn.EnableReadFrames(true)
			conn.EnableReadTimeout(true)

			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Session": base.HeaderValue{"12345678"},
				},
			}, nil

		// The Teardown method is called to close a session
		case base.Teardown:
			return &base.Response{
				StatusCode: base.StatusOK,
			}, fmt.Errorf("terminated")
		}

		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("unhandled method: %v", req.Method)
	}

	// this is called when a frame arrives
	onFrame := func(trackID int, typ gortsplib.StreamType, buf []byte) {
		mutex.Lock()
		defer mutex.Unlock()

		// if we are the publisher, route frames to readers
		if conn == publisher {
			for r := range readers {
				r.WriteFrame(trackID, typ, buf)
			}
		}
	}

	err := <-conn.Read(onRequest, onFrame)
	log.Printf("client disconnected (%s)", err)

	mutex.Lock()
	defer mutex.Unlock()

	if conn == publisher {
		publisher = nil
		sdp = nil
	}
}

func main() {
	// create server
	s, err := gortsplib.Serve(":8554")
	if err != nil {
		panic(err)
	}
	log.Printf("server is ready")

	// accept connections
	for {
		conn, err := s.Accept()
		if err != nil {
			panic(err)
		}

		go handleConn(conn)
	}
}
