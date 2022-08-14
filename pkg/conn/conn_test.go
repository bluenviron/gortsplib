package conn

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

func TestReadInterleavedFrameOrRequest(t *testing.T) {
	byts := []byte("DESCRIBE rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
		"Accept: application/sdp\r\n" +
		"CSeq: 2\r\n" +
		"\r\n")
	byts = append(byts, []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4}...)

	var f base.InterleavedFrame
	var req base.Request
	conn := NewConn(bytes.NewBuffer(byts))

	out, err := conn.ReadInterleavedFrameOrRequest(&f, &req)
	require.NoError(t, err)
	require.Equal(t, &req, out)

	out, err = conn.ReadInterleavedFrameOrRequest(&f, &req)
	require.NoError(t, err)
	require.Equal(t, &f, out)
}

func TestReadInterleavedFrameOrRequestErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
		err  string
	}{
		{
			"empty",
			[]byte{},
			"EOF",
		},
		{
			"invalid frame",
			[]byte{0x24, 0x00},
			"unexpected EOF",
		},
		{
			"invalid request",
			[]byte("DESCRIBE"),
			"EOF",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var f base.InterleavedFrame
			var req base.Request
			conn := NewConn(bytes.NewBuffer(ca.byts))
			_, err := conn.ReadInterleavedFrameOrRequest(&f, &req)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestReadInterleavedFrameOrResponse(t *testing.T) {
	byts := []byte("RTSP/1.0 200 OK\r\n" +
		"CSeq: 1\r\n" +
		"Public: DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE\r\n" +
		"\r\n")
	byts = append(byts, []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4}...)

	var f base.InterleavedFrame
	var res base.Response
	conn := NewConn(bytes.NewBuffer(byts))

	out, err := conn.ReadInterleavedFrameOrResponse(&f, &res)
	require.NoError(t, err)
	require.Equal(t, &res, out)

	out, err = conn.ReadInterleavedFrameOrResponse(&f, &res)
	require.NoError(t, err)
	require.Equal(t, &f, out)
}

func TestReadInterleavedFrameOrResponseErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
		err  string
	}{
		{
			"empty",
			[]byte{},
			"EOF",
		},
		{
			"invalid frame",
			[]byte{0x24, 0x00},
			"unexpected EOF",
		},
		{
			"invalid response",
			[]byte("RTSP/1.0"),
			"EOF",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var f base.InterleavedFrame
			var res base.Response
			conn := NewConn(bytes.NewBuffer(ca.byts))
			_, err := conn.ReadInterleavedFrameOrResponse(&f, &res)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestReadRequestIgnoreFrames(t *testing.T) {
	byts := []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4}
	byts = append(byts, []byte("OPTIONS rtsp://example.com/media.mp4 RTSP/1.0\r\n"+
		"CSeq: 1\r\n"+
		"Proxy-Require: gzipped-messages\r\n"+
		"Require: implicit-play\r\n"+
		"\r\n")...)

	conn := NewConn(bytes.NewBuffer(byts))
	var req base.Request
	err := conn.ReadRequestIgnoreFrames(&req)
	require.NoError(t, err)
}

func TestReadRequestIgnoreFramesErrors(t *testing.T) {
	byts := []byte{0x25}

	conn := NewConn(bytes.NewBuffer(byts))
	var req base.Request
	err := conn.ReadRequestIgnoreFrames(&req)
	require.EqualError(t, err, "EOF")
}

func TestReadResponseIgnoreFrames(t *testing.T) {
	byts := []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4}
	byts = append(byts, []byte("RTSP/1.0 200 OK\r\n"+
		"CSeq: 1\r\n"+
		"Public: DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE\r\n"+
		"\r\n")...)

	conn := NewConn(bytes.NewBuffer(byts))
	var res base.Response
	err := conn.ReadResponseIgnoreFrames(&res)
	require.NoError(t, err)
}

func TestReadResponseIgnoreFramesErrors(t *testing.T) {
	byts := []byte{0x25}

	conn := NewConn(bytes.NewBuffer(byts))
	var res base.Response
	err := conn.ReadResponseIgnoreFrames(&res)
	require.EqualError(t, err, "EOF")
}
