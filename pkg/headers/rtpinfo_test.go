package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func uintPtr(v uint) *uint {
	return &v
}

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
				URL:            "rtsp://127.0.0.1/test.mkv/track1",
				SequenceNumber: uint16Ptr(35243),
				Timestamp:      uint32Ptr(717574556),
			},
		},
	},
	{
		"multiple value",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556,` +
			`url=rtsp://127.0.0.1/test.mkv/track2;seq=13655;rtptime=2848846950`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=717574556,` +
			`url=rtsp://127.0.0.1/test.mkv/track2;seq=13655;rtptime=2848846950`},
		RTPInfo{
			{
				URL:            "rtsp://127.0.0.1/test.mkv/track1",
				SequenceNumber: uint16Ptr(35243),
				Timestamp:      uint32Ptr(717574556),
			},
			{
				URL:            "rtsp://127.0.0.1/test.mkv/track2",
				SequenceNumber: uint16Ptr(13655),
				Timestamp:      uint32Ptr(2848846950),
			},
		},
	},
	{
		"missing timestamp",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243`},
		RTPInfo{
			{
				URL:            "rtsp://127.0.0.1/test.mkv/track1",
				SequenceNumber: uint16Ptr(35243),
			},
		},
	},
	{
		"missing sequence number",
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;rtptime=717574556`},
		base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;rtptime=717574556`},
		RTPInfo{
			{
				URL:       "rtsp://127.0.0.1/test.mkv/track1",
				Timestamp: uint32Ptr(717574556),
			},
		},
	},
	{
		"path instead of url",
		base.HeaderValue{`url=trackID=0;seq=12447;rtptime=12447`},
		base.HeaderValue{`url=trackID=0;seq=12447;rtptime=12447`},
		RTPInfo{
			{
				URL:            "trackID=0",
				SequenceNumber: uint16Ptr(12447),
				Timestamp:      uint32Ptr(12447),
			},
		},
	},
	{
		"with space",
		base.HeaderValue{`url=rtsp://10.13.146.53/axis-media/media.amp/trackID=1;` +
			`seq=58477;rtptime=1020884293, url=rtsp://10.13.146.53/axis-media/media.amp/trackID=2;seq=15727;rtptime=1171661503`},
		base.HeaderValue{`url=rtsp://10.13.146.53/axis-media/media.amp/trackID=1;` +
			`seq=58477;rtptime=1020884293,url=rtsp://10.13.146.53/axis-media/media.amp/trackID=2;seq=15727;rtptime=1171661503`},
		RTPInfo{
			{
				URL:            "rtsp://10.13.146.53/axis-media/media.amp/trackID=1",
				SequenceNumber: uint16Ptr(58477),
				Timestamp:      uint32Ptr(1020884293),
			},
			{
				URL:            "rtsp://10.13.146.53/axis-media/media.amp/trackID=2",
				SequenceNumber: uint16Ptr(15727),
				Timestamp:      uint32Ptr(1171661503),
			},
		},
	},
	{
		"with session",
		base.HeaderValue{`url=trackID=1;seq=55664;rtptime=254718369;ssrc=56597976,` +
			`url=trackID=2;seq=43807;rtptime=1702259566;ssrc=ee839a80`},
		base.HeaderValue{`url=trackID=1;seq=55664;rtptime=254718369,` +
			`url=trackID=2;seq=43807;rtptime=1702259566`},
		RTPInfo{
			{
				URL:            "trackID=1",
				SequenceNumber: uint16Ptr(55664),
				Timestamp:      uint32Ptr(254718369),
			},
			{
				URL:            "trackID=2",
				SequenceNumber: uint16Ptr(43807),
				Timestamp:      uint32Ptr(1702259566),
			},
		},
	},
}

func TestRTPInfoUnmarshal(t *testing.T) {
	for _, ca := range casesRTPInfo {
		t.Run(ca.name, func(t *testing.T) {
			var h RTPInfo
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestRTPInfoUnmarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		hv   base.HeaderValue
		err  string
	}{
		{
			"empty",
			base.HeaderValue{},
			"value not provided",
		},
		{
			"2 values",
			base.HeaderValue{"a", "b"},
			"value provided multiple times ([a b])",
		},
		{
			"invalid key-value",
			base.HeaderValue{"test=\"a"},
			"apexes not closed (test=\"a)",
		},
		{
			"invalid sequence",
			base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=aa;rtptime=717574556`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"invalid rtptime",
			base.HeaderValue{`url=rtsp://127.0.0.1/test.mkv/track1;seq=35243;rtptime=aa`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"missing URL",
			base.HeaderValue{`seq=35243;rtptime=717574556`},
			"URL is missing",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var h RTPInfo
			err := h.Unmarshal(ca.hv)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestRTPInfoMarshal(t *testing.T) {
	for _, ca := range casesRTPInfo {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Marshal()
			require.Equal(t, ca.vout, req)
		})
	}
}
