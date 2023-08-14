package rtpsimpleaudio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			frame, err := d.Decode(ca.pkt)
			require.NoError(t, err)
			require.Equal(t, ca.frame, frame)
		})
	}
}
