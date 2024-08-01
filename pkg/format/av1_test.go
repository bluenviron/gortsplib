package format //nolint:dupl

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestAV1Attributes(t *testing.T) {
	format := &AV1{
		PayloadTyp: 100,
	}
	require.Equal(t, "AV1", format.Codec())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestAV1DecEncoder(t *testing.T) {
	format := &AV1{}

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

func FuzzUnmarshalAV1(f *testing.F) {
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
			ma["level-idx"] = b
		}

		if c {
			ma["profile"] = d
		}

		if e {
			ma["tier"] = f
		}

		Unmarshal("video", 96, "AV1/90000", ma) //nolint:errcheck
	})
}
