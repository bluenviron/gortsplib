package format

import (
	"testing"

	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestH265Attributes(t *testing.T) {
	format := &H265{
		PayloadTyp: 96,
		VPS:        []byte{0x01, 0x02},
		SPS:        []byte{0x03, 0x04},
		PPS:        []byte{0x05, 0x06},
	}
	require.Equal(t, "H265", format.Codec())
	require.Equal(t, 90000, format.ClockRate())

	vps, sps, pps := format.SafeParams()
	require.Equal(t, []byte{0x01, 0x02}, vps)
	require.Equal(t, []byte{0x03, 0x04}, sps)
	require.Equal(t, []byte{0x05, 0x06}, pps)

	format.SafeSetParams([]byte{0x07, 0x08}, []byte{0x09, 0x0A}, []byte{0x0B, 0x0C})

	vps, sps, pps = format.SafeParams()
	require.Equal(t, []byte{0x07, 0x08}, vps)
	require.Equal(t, []byte{0x09, 0x0A}, sps)
	require.Equal(t, []byte{0x0B, 0x0C}, pps)
}

func TestH265PTSEqualsDTS(t *testing.T) {
	format := &H265{
		PayloadTyp: 96,
		VPS:        []byte{0x01, 0x02},
		SPS:        []byte{0x03, 0x04},
		PPS:        []byte{0x05, 0x06},
	}

	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{
		Payload: []byte{byte(h265.NALUType_CRA_NUT) << 1},
	}))

	// CRA_NUT inside FragmentationUnit
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{
		Payload: []byte{0x62, 0x1, 0x95, 0xaf, 0xe8},
	}))

	require.Equal(t, false, format.PTSEqualsDTS(&rtp.Packet{
		Payload: []byte{byte(h265.NALUType_TRAIL_N) << 1},
	}))
}

func TestH265DecEncoder(t *testing.T) {
	format := &H265{}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	pkts, err := enc.Encode([][]byte{{0x01, 0x02, 0x03, 0x04}})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x02, 0x03, 0x04}}, byts)
}

func FuzzUnmarshalH265(f *testing.F) {
	f.Fuzz(func(
		_ *testing.T,
		a bool,
		b string,
		c bool,
		d string,
	) {
		ma := map[string]string{}

		if a {
			ma["sprop-vps"] = b
		}

		if c {
			ma["sprop-sps"] = d
		}

		if c {
			ma["sprop-pps"] = d
		}

		if c {
			ma["sprop-max-don-diff"] = d
		}

		fo, err := Unmarshal("video", 96, "H265/90000", ma)
		if err == nil {
			fo.RTPMap()
			fo.FMTP()
		}
	})
}

func FuzzH265PTSEqualsDTS(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		(&H265{}).PTSEqualsDTS(&rtp.Packet{Payload: b})
	})
}
