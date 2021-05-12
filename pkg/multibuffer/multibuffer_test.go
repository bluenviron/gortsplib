package multibuffer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	mb := New(2, 4)

	b := mb.Next()
	copy(b, []byte{0x01, 0x02, 0x03, 0x04})

	b = mb.Next()
	copy(b, []byte{0x05, 0x06, 0x07, 0x08})

	b = mb.Next()
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, b)

	b = mb.Next()
	require.Equal(t, []byte{0x05, 0x06, 0x07, 0x08}, b)
}
