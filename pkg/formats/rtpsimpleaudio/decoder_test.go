package rtpsimpleaudio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				SampleRate: 8000,
			}
			err := d.Init()
			require.NoError(t, err)

			frame, _, err := d.Decode(ca.pkt)
			require.NoError(t, err)
			require.Equal(t, ca.frame, frame)
		})
	}
}
