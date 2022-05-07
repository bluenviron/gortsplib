package h264

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAntiCompetitionRemove(t *testing.T) {
	for _, ca := range []struct {
		name   string
		unproc []byte
		proc   []byte
	}{
		{
			"base",
			[]byte{
				0x00, 0x00, 0x00,
				0x00, 0x00, 0x01,
				0x00, 0x00, 0x02,
				0x00, 0x00, 0x03,
			},
			[]byte{
				0x00, 0x00, 0x03, 0x00,
				0x00, 0x00, 0x03, 0x01,
				0x00, 0x00, 0x03, 0x02,
				0x00, 0x00, 0x03, 0x03,
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			unproc := AntiCompetitionRemove(ca.proc)
			require.Equal(t, ca.unproc, unproc)
		})
	}
}
