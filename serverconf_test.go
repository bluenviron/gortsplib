package gortsplib

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

type testServ struct {
	s         *Server
	wg        sync.WaitGroup
	mutex     sync.Mutex
	publisher *ServerConn
	sdp       []byte
	readers   map[*ServerConn]struct{}
}

func newTestServ() (*testServ, error) {
	s, err := Serve(":8554")
	if err != nil {
		return nil, err
	}

	ts := &testServ{
		s:       s,
		readers: make(map[*ServerConn]struct{}),
	}

	ts.wg.Add(1)
	go ts.run()

	return ts, nil
}

func (ts *testServ) close() {
	ts.s.Close()
	ts.wg.Wait()
}

func (ts *testServ) run() {
	defer ts.wg.Done()

	for {
		conn, err := ts.s.Accept()
		if err != nil {
			return
		}

		ts.wg.Add(1)
		go ts.handleConn(conn)
	}
}

func (ts *testServ) handleConn(conn *ServerConn) {
	defer ts.wg.Done()
	defer conn.Close()

	// this is called when a request arrives
	onRequest := func(req *base.Request) (*base.Response, error) {
		switch req.Method {
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

		case base.Describe:
			ts.mutex.Lock()
			defer ts.mutex.Unlock()

			if ts.publisher == nil {
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
				Content: ts.sdp,
			}, nil

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

			tracks, err := ReadTracks(req.Content)
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

			ts.mutex.Lock()
			defer ts.mutex.Unlock()

			if ts.publisher != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("someone is already publishing")
			}

			ts.publisher = conn
			ts.sdp = tracks.Write()

			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Session": base.HeaderValue{"12345678"},
				},
			}, nil

		case base.Setup:
			th, err := headers.ReadTransport(req.Header["Transport"])
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("transport header: %s", err)
			}

			if th.Protocol == StreamProtocolUDP {
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

		case base.Play:
			ts.mutex.Lock()
			defer ts.mutex.Unlock()

			ts.readers[conn] = struct{}{}

			conn.EnableReadFrames(true)
			conn.EnableReadTimeout(false)

			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Session": base.HeaderValue{"12345678"},
				},
			}, nil

		case base.Record:
			ts.mutex.Lock()
			defer ts.mutex.Unlock()

			if conn != ts.publisher {
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

		case base.Teardown:
			return &base.Response{
				StatusCode: base.StatusOK,
			}, fmt.Errorf("terminated")
		}

		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("unhandled method: %v", req.Method)
	}

	onFrame := func(trackID int, typ StreamType, buf []byte) {
		ts.mutex.Lock()
		defer ts.mutex.Unlock()

		if conn == ts.publisher {
			for r := range ts.readers {
				r.WriteFrame(trackID, typ, buf)
			}
		}
	}

	<-conn.Read(onRequest, onFrame)

	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if conn == ts.publisher {
		ts.publisher = nil
		ts.sdp = nil
	}
}

func TestServerPublishReadTCP(t *testing.T) {
	ts, err := newTestServ()
	require.NoError(t, err)
	defer ts.close()

	cnt1, err := newContainer("ffmpeg", "publish", []string{
		"-re",
		"-stream_loop", "-1",
		"-i", "/emptyvideo.ts",
		"-c", "copy",
		"-f", "rtsp",
		"-rtsp_transport", "tcp",
		"rtsp://localhost:8554/teststream",
	})
	require.NoError(t, err)
	defer cnt1.close()

	time.Sleep(1 * time.Second)

	cnt2, err := newContainer("ffmpeg", "read", []string{
		"-rtsp_transport", "tcp",
		"-i", "rtsp://localhost:8554/teststream",
		"-vframes", "1",
		"-f", "image2",
		"-y", "/dev/null",
	})
	require.NoError(t, err)
	defer cnt2.close()

	require.Equal(t, 0, cnt2.wait())
}
