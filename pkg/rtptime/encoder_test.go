package rtptime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func TestEncoder(t *testing.T) {
	e := &Encoder{
		ClockRate:        90000,
		InitialTimestamp: uint32Ptr(12345),
	}
	err := e.Initialize()
	require.NoError(t, err)

	ts := e.Encode(0)
	require.Equal(t, uint32(12345), ts)

	ts = e.Encode(3 * time.Second / 2)
	require.Equal(t, uint32(12345+135000), ts)
}

func BenchmarkEncoder(b *testing.B) {
	e := &Encoder{
		ClockRate:        90000,
		InitialTimestamp: uint32Ptr(12345),
	}
	e.Initialize() //nolint:errcheck

	for i := 0; i < b.N; i++ {
		e.Encode(200 * time.Second)
	}
}
