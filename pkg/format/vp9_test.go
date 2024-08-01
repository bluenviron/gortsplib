package format //nolint:dupl

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestVP9Attributes(t *testing.T) {
	format := &VP9{
		PayloadTyp: 100,
	}
	require.Equal(t, "VP9", format.Codec())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestVP9DecEncoder(t *testing.T) {
	format := &VP9{}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	pkts, err := enc.Encode([]byte{0x82, 0x49, 0x83, 0x42, 0x0, 0x77, 0xf0, 0x32, 0x34})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, []byte{0x82, 0x49, 0x83, 0x42, 0x0, 0x77, 0xf0, 0x32, 0x34}, byts)
}

func FuzzUnmarshalVP9(f *testing.F) {
	f.Fuzz(func(
		_ *testing.T,
		a bool,
		b string,
		c bool,
		d string,
		e bool,
		f string,
	) {
		ma := map[string]string{}

		if a {
			ma["max-fr"] = b
		}

		if c {
			ma["max-fs"] = d
		}

		if e {
			ma["profile-id"] = f
		}

		fo, err := Unmarshal("audio", 96, "VP9/90000", ma)
		if err == nil {
			fo.RTPMap()
			fo.FMTP()
		}
	})
}
