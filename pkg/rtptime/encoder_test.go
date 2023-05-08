package rtptime

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEncoder(t *testing.T) {
	e := NewEncoder(90000, 12345)

	ts := e.Encode(0)
	require.Equal(t, uint32(12345), ts)

	ts = e.Encode(3 * time.Second / 2)
	require.Equal(t, uint32(12345+135000), ts)
}

func BenchmarkEncoder(b *testing.B) {
	for i := 0; i < b.N; i++ {
		func() {
			d := NewEncoder(90000, 0)
			d.Encode(200 * time.Second)
		}()
	}
}
