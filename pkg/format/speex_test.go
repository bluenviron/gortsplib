package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestSpeexAttributes(t *testing.T) {
	format := &Speex{
		PayloadTyp: 96,
		SampleRate: 16000,
	}
	require.Equal(t, "Speex", format.Codec())
	require.Equal(t, 16000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
