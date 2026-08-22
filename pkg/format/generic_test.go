package format_test //nolint:revive

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

func TestGenericAttributes(t *testing.T) {
	format := &format.Generic{
		PayloadTyp: 98,
		RTPMa:      "H265/90000",
		FMT: map[string]string{
			"profile-id": "1",
			"sprop-vps":  "QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ",
			"sprop-sps":  "QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=",
			"sprop-pps":  "RAHgdrAwxmQ=",
		},
	}
	err := format.Init()
	require.NoError(t, err)

	require.Equal(t, "Generic", format.Codec())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
