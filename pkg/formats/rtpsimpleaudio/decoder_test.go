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
			d.Init()

			frame, _, err := d.Decode(ca.pkt)
			require.NoError(t, err)
			require.Equal(t, ca.frame, frame)
		})
	}
}
