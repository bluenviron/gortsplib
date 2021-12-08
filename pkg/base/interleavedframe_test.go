package base

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesInterleavedFrame = []struct {
	name string
	enc  []byte
	dec  InterleavedFrame
}{
	{
		name: "rtp",
		enc:  []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4},
		dec: InterleavedFrame{
			Channel: 6,
			Payload: []byte{0x01, 0x02, 0x03, 0x04},
		},
	},
	{
		name: "rtcp",
		enc:  []byte{0x24, 0xd, 0x0, 0x4, 0x5, 0x6, 0x7, 0x8},
		dec: InterleavedFrame{
			Channel: 13,
			Payload: []byte{0x05, 0x06, 0x07, 0x08},
		},
	},
}

func TestInterleavedFrameRead(t *testing.T) {
	// keep f global to make sure that all its fields are overridden.
	var f InterleavedFrame

	for _, ca := range casesInterleavedFrame {
		t.Run(ca.name, func(t *testing.T) {
			f.Payload = make([]byte, 1024)
			err := f.Read(bufio.NewReader(bytes.NewBuffer(ca.enc)))
			require.NoError(t, err)
			require.Equal(t, ca.dec, f)
		})
	}
}

func TestInterleavedFrameReadErrors(t *testing.T) {
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
			"invalid magic byte",
			[]byte{0x55, 0x00, 0x00, 0x00},
			"invalid magic byte (0x55)",
		},
		{
			"payload size too big",
			[]byte{0x24, 0x00, 0x00, 0x08},
			"payload size greater than maximum allowed (8 vs 5)",
		},
		{
			"payload invalid",
			[]byte{0x24, 0x00, 0x00, 0x05, 0x01, 0x02},
			"unexpected EOF",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var f InterleavedFrame
			f.Payload = make([]byte, 5)
			err := f.Read(bufio.NewReader(bytes.NewBuffer(ca.byts)))
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestInterleavedFrameWrite(t *testing.T) {
	for _, ca := range casesInterleavedFrame {
		t.Run(ca.name, func(t *testing.T) {
			var buf bytes.Buffer
			ca.dec.Write(&buf)
			require.Equal(t, ca.enc, buf.Bytes())
		})
	}
}

func TestReadInterleavedFrameOrRequest(t *testing.T) {
	byts := []byte("DESCRIBE rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
		"Accept: application/sdp\r\n" +
		"CSeq: 2\r\n" +
		"\r\n")
	byts = append(byts, []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4}...)

	var f InterleavedFrame
	f.Payload = make([]byte, 10)
	var req Request
	br := bufio.NewReader(bytes.NewBuffer(byts))

	out, err := ReadInterleavedFrameOrRequest(&f, &req, br)
	require.NoError(t, err)
	require.Equal(t, &req, out)

	out, err = ReadInterleavedFrameOrRequest(&f, &req, br)
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
			var f InterleavedFrame
			f.Payload = make([]byte, 10)
			var req Request
			br := bufio.NewReader(bytes.NewBuffer(ca.byts))

			_, err := ReadInterleavedFrameOrRequest(&f, &req, br)
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

	var f InterleavedFrame
	f.Payload = make([]byte, 10)
	var res Response
	br := bufio.NewReader(bytes.NewBuffer(byts))

	out, err := ReadInterleavedFrameOrResponse(&f, &res, br)
	require.NoError(t, err)
	require.Equal(t, &res, out)

	out, err = ReadInterleavedFrameOrResponse(&f, &res, br)
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
			var f InterleavedFrame
			f.Payload = make([]byte, 10)
			var res Response
			br := bufio.NewReader(bytes.NewBuffer(ca.byts))

			_, err := ReadInterleavedFrameOrResponse(&f, &res, br)
			require.EqualError(t, err, ca.err)
		})
	}
}
