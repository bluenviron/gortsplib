package format_test //nolint:revive

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

func TestSpeexAttributes(t *testing.T) {
	format := &format.Speex{
		PayloadTyp: 96,
		SampleRate: 16000,
	}
	require.Equal(t, "Speex", format.Codec())
	require.Equal(t, 16000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
