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

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Session":      base.HeaderValue{"123456"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	u, err := base.ParseURL("rtsp://localhost:8554/stream")
	require.NoError(t, err)

	conn, err := Dial(u.Scheme, u.Host)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	_, _, err = conn.Describe(u)
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

		err = v.ValidateRequest(req, nil)
		require.NoError(t, err)

		track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
		require.NoError(t, err)

		err = base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: Tracks{track}.Write(),
		}.Write(bconn.Writer)
		require.NoError(t, err)
	}()

	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/stream")
	require.NoError(t, err)

	conn, err := Dial(u.Scheme, u.Host)
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	_, _, err = conn.Describe(u)
	require.NoError(t, err)
}
