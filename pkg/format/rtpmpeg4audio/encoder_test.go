package rtpmpeg4audio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{
		PayloadType:      96,
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	err := e.Init()
	require.NoError(t, err)
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
}
