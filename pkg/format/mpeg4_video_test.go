package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEG4VideoAttributes(t *testing.T) {
	format := &MPEG4Video{
		PayloadTyp:     96,
		ProfileLevelID: 1,
		Config:         []byte{0x01, 0x02, 0x03},
	}
	require.Equal(t, "MPEG-4 Video", format.Codec())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEG4VideoDecEncoder(t *testing.T) {
	format := &MPEG4Video{
		PayloadTyp: 96,
	}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	pkts, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}

func FuzzUnmarshalMPEG4Video(f *testing.F) {
	f.Fuzz(func(
		_ *testing.T,
		a bool,
		b string,
		c bool,
		d string,
	) {
		ma := map[string]string{}

		if a {
			ma["profile-level-id"] = b
		}

		if c {
			ma["config"] = d
		}

		fo, err := Unmarshal("audio", 96, "MP4V-ES/90000", ma)
		if err == nil {
			fo.RTPMap()
			fo.FMTP()
		}
	})
}
