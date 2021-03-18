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
	h    *RTPInfo
}{
	{
		"single value",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556`},
		&RTPInfo{
			{
				URL:            base.MustParseURL("rtsp://127.0.0.1/test.mkv/track1"),
				SequenceNumber: 35243,
				Timestamp:      717574556,
			},
		},
	},
	{
		"multiple value",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556,url=rtsp://127.0.0.1/test.mkv/track2;seq=13655;rtptime=2848846950`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556,url=rtsp://127.0.0.1/test.mkv/track2;seq=13655;rtptime=2848846950`},
		&RTPInfo{
			{
				URL:            base.MustParseURL("rtsp://127.0.0.1/test.mkv/track1"),
				SequenceNumber: 35243,
				Timestamp:      717574556,
			},
			{
				URL:            base.MustParseURL("rtsp://127.0.0.1/test.mkv/track2"),
				SequenceNumber: 13655,
				Timestamp:      2848846950,
			},
		},
	},
}

func TestRTPInfoRead(t *testing.T) {
	for _, c := range casesRTPInfo {
		t.Run(c.name, func(t *testing.T) {
			req, err := ReadRTPInfo(c.vin)
			require.NoError(t, err)
			require.Equal(t, c.h, req)
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
