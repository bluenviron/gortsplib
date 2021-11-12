package gortsplib

import (
	"bufio"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/auth"
	"github.com/aler9/gortsplib/pkg/base"
)

func mustParseURL(s string) *base.URL {
	u, err := base.ParseURL(s)
	if err != nil {
		panic(err)
	}
	return u
}

func readRequest(br *bufio.Reader) (*base.Request, error) {
	var req base.Request
	err := req.Read(br)
	return &req, err
}

func readRequestIgnoreFrames(br *bufio.Reader) (*base.Request, error) {
	buf := make([]byte, 2048)
	var req base.Request
	err := req.ReadIgnoreFrames(br, buf)
	return &req, err
}

func TestClientSession(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		defer conn.Close()

		req, err := readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
				"Session": base.HeaderValue{"123456"},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		require.Equal(t, base.HeaderValue{"123456"}, req.Header["Session"])

		track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
		require.NoError(t, err)

		tracks := cloneAndClearTracks(Tracks{track})

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Session":      base.HeaderValue{"123456"},
			},
			Body: tracks.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	u, err := base.ParseURL("rtsp://localhost:8554/stream")
	require.NoError(t, err)

	c := Client{}

	conn, err := c.Dial(u.Scheme, u.Host)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	_, _, _, err = conn.Describe(u)
	require.NoError(t, err)
}

func TestClientAuth(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		defer conn.Close()

		req, err := readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		v := auth.NewValidator("myuser", "mypass", nil)

		err = base.Response{
			StatusCode: base.StatusUnauthorized,
			Header: base.Header{
				"WWW-Authenticate": v.Header(),
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		err = v.ValidateRequest(req)
		require.NoError(t, err)

		track, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
		require.NoError(t, err)

		tracks := cloneAndClearTracks(Tracks{track})

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: tracks.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/stream")
	require.NoError(t, err)

	c := Client{}

	conn, err := c.Dial(u.Scheme, u.Host)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	_, _, _, err = conn.Describe(u)
	require.NoError(t, err)
}

func TestClientDescribeCharset(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		conn, err := l.Accept()
		require.NoError(t, err)
		defer conn.Close()
		bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

		req, err := readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
			},
		}.Write(bconn.Writer)
		require.NoError(t, err)

		req, err = readRequest(bconn.Reader)
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		track1, err := NewTrackH264(96, &TrackConfigH264{[]byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}})
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp; charset=utf-8"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: Tracks{track1}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{}

	conn, err := c.Dial(u.Scheme, u.Host)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	_, _, _, err = conn.Describe(u)
	require.NoError(t, err)
}
