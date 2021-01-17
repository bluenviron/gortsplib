// +build ignore

package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"sync"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// This example shows how to
// 1. create a RTSP server which accepts only connections encrypted with TLS (RTSPS)
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

	// called after receiving a DESCRIBE request.
	onDescribe := func(req *base.Request) (*base.Response, error) {
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
			Body: sdp,
		}, nil
	}

	// called after receiving an ANNOUNCE request.
	onAnnounce := func(req *base.Request, tracks gortsplib.Tracks) (*base.Response, error) {
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
	}

	// called after receiving a SETUP request.
	onSetup := func(req *base.Request, th *headers.Transport, basePath string, trackID int) (*base.Response, error) {
		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Session": base.HeaderValue{"12345678"},
			},
		}, nil
	}

	// called after receiving a PLAY request.
	onPlay := func(req *base.Request) (*base.Response, error) {
		mutex.Lock()
		defer mutex.Unlock()

		readers[conn] = struct{}{}

		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Session": base.HeaderValue{"12345678"},
			},
		}, nil
	}

	// called after receiving a RECORD request.
	onRecord := func(req *base.Request) (*base.Response, error) {
		mutex.Lock()
		defer mutex.Unlock()

		if conn != publisher {
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

	// called after receiving a Frame.
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

	err := <-conn.Read(gortsplib.ServerConnReadHandlers{
		OnDescribe: onDescribe,
		OnAnnounce: onAnnounce,
		OnSetup:    onSetup,
		OnPlay:     onPlay,
		OnRecord:   onRecord,
		OnFrame:    onFrame,
	})
	log.Printf("client disconnected (%s)", err)

	mutex.Lock()
	defer mutex.Unlock()

	if conn == publisher {
		publisher = nil
		sdp = nil
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
	conf := gortsplib.ServerConf{
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
	}

	// create server
	s, err := conf.Serve(":8554")
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
