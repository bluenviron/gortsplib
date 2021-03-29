package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

var casesRTPInfo = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    RTPInfo
}{
	{
		"single value",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556`},
		RTPInfo{
			{
				URL: "rtsp://127.0.0.1/test.mkv/track1",
				SequenceNumber: func() *uint16 {
					v := uint16(35243)
					return &v
				}(),
				Timestamp: func() *uint32 {
					v := uint32(717574556)
					return &v
				}(),
			},
		},
	},
	{
		"multiple value",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556,url=rtsp://127.0.0.1/test.mkv/track2;seq=13655;rtptime=2848846950`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556,url=rtsp://127.0.0.1/test.mkv/track2;seq=13655;rtptime=2848846950`},
		RTPInfo{
			{
				URL: "rtsp://127.0.0.1/test.mkv/track1",
				SequenceNumber: func() *uint16 {
					v := uint16(35243)
					return &v
				}(),
				Timestamp: func() *uint32 {
					v := uint32(717574556)
					return &v
				}(),
			},
			{
				URL: "rtsp://127.0.0.1/test.mkv/track2",
				SequenceNumber: func() *uint16 {
					v := uint16(13655)
					return &v
				}(),
				Timestamp: func() *uint32 {
					v := uint32(2848846950)
					return &v
				}(),
			},
		},
	},
	{
		"missing timestamp",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243`},
		RTPInfo{
			{
				URL: "rtsp://127.0.0.1/test.mkv/track1",
				SequenceNumber: func() *uint16 {
					v := uint16(35243)
					return &v
				}(),
			},
		},
	},
	{
		"missing sequence number",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;rtptime=717574556`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;rtptime=717574556`},
		RTPInfo{
			{
				URL: "rtsp://127.0.0.1/test.mkv/track1",
				Timestamp: func() *uint32 {
					v := uint32(717574556)
					return &v
				}(),
			},
		},
	},
	{
		"path instead of url",
		base.HeaderValue{`url=trackID=0;seq=12447;rtptime=12447`},
		base.HeaderValue{`url=trackID=0;seq=12447;rtptime=12447`},
		RTPInfo{
			{
				URL: "trackID=0",
				SequenceNumber: func() *uint16 {
					v := uint16(12447)
					return &v
				}(),
				Timestamp: func() *uint32 {
					v := uint32(12447)
					return &v
				}(),
			},
		},
	},
}

func TestRTPInfoRead(t *testing.T) {
	for _, c := range casesRTPInfo {
		t.Run(c.name, func(t *testing.T) {
			var h RTPInfo
			err := h.Read(c.vin)
			require.NoError(t, err)
			require.Equal(t, c.h, h)
		})
	}
}

func TestRTPInfoWrite(t *testing.T) {
	for _, c := range casesRTPInfo {
		t.Run(c.name, func(t *testing.T) {
			req := c.h.Write()
			require.Equal(t, c.vout, req)
		})
	}
}
