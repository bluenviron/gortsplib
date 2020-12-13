package gortsplib

import (
	"bufio"
	"fmt"
	"io"
	"net"
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

	onDescribe := func(req *base.Request) (*base.Response, error) {
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
	}

	onAnnounce := func(req *base.Request, tracks Tracks) (*base.Response, error) {
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
	}

	onSetup := func(req *base.Request, th *headers.Transport) (*base.Response, error) {
		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Transport": req.Header["Transport"],
				"Session":   base.HeaderValue{"12345678"},
			},
		}, nil
	}

	onPlay := func(req *base.Request) (*base.Response, error) {
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
	}

	onRecord := func(req *base.Request) (*base.Response, error) {
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

	<-conn.Read(ServerConnReadHandlers{
		OnDescribe: onDescribe,
		OnAnnounce: onAnnounce,
		OnSetup:    onSetup,
		OnPlay:     onPlay,
		OnRecord:   onRecord,
		OnFrame:    onFrame,
	})

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

func TestServerTeardown(t *testing.T) {
	ts, err := newTestServ()
	require.NoError(t, err)
	defer ts.close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	req := base.Request{
		Method: base.Teardown,
		URL:    base.MustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	}
	err = req.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	buf := make([]byte, 2048)
	_, err = bconn.Read(buf)
	require.Equal(t, io.EOF, err)
}
