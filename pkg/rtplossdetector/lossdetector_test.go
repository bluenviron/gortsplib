package rtplossdetector

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestLossDetector(t *testing.T) {
	d := New()

	c := d.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 65530,
		},
	})
	require.Equal(t, 0, c)

	c = d.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 65531,
		},
	})
	require.Equal(t, 0, c)

	c = d.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 65535,
		},
	})
	require.Equal(t, 3, c)
}
