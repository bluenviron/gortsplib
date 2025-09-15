package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
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
				URL:            "rtsp://127.0.0.1/test.mkv/track1",
				SequenceNumber: ptrOf(uint16(35243)),
				Timestamp:      ptrOf(uint32(717574556)),
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
				SequenceNumber: ptrOf(uint16(35243)),
				Timestamp:      ptrOf(uint32(717574556)),
			},
			{
				URL:            "rtsp://127.0.0.1/test.mkv/track2",
				SequenceNumber: ptrOf(uint16(13655)),
				Timestamp:      ptrOf(uint32(2848846950)),
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
				SequenceNumber: ptrOf(uint16(35243)),
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
				Timestamp: ptrOf(uint32(717574556)),
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
				SequenceNumber: ptrOf(uint16(12447)),
				Timestamp:      ptrOf(uint32(12447)),
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
				SequenceNumber: ptrOf(uint16(58477)),
				Timestamp:      ptrOf(uint32(1020884293)),
			},
			{
				URL:            "rtsp://10.13.146.53/axis-media/media.amp/trackID=2",
				SequenceNumber: ptrOf(uint16(15727)),
				Timestamp:      ptrOf(uint32(1171661503)),
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
				SequenceNumber: ptrOf(uint16(55664)),
				Timestamp:      ptrOf(uint32(254718369)),
			},
			{
				URL:            "trackID=2",
				SequenceNumber: ptrOf(uint16(43807)),
				Timestamp:      ptrOf(uint32(1702259566)),
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

func TestRTPInfoMarshal(t *testing.T) {
	for _, ca := range casesRTPInfo {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Marshal()
			require.Equal(t, ca.vout, req)
		})
	}
}

func FuzzRTPInfoUnmarshal(f *testing.F) {
	for _, ca := range casesRTPInfo {
		f.Add(ca.vin[0])
	}

	f.Fuzz(func(_ *testing.T, b string) {
		var h RTPInfo
		err := h.Unmarshal(base.HeaderValue{b})
		if err != nil {
			return
		}

		h.Marshal()
	})
}

func TestRTPInfoAdditionalErrors(t *testing.T) {
	func() {
		var h RTPInfo
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h RTPInfo
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
