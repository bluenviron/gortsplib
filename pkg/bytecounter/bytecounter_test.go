package bytecounter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByteCounter(t *testing.T) {
	bc := New(bytes.NewBuffer(nil))

	bc.Write([]byte{0x01, 0x02, 0x03, 0x04})

	buf := make([]byte, 2)
	bc.Read(buf)

	require.Equal(t, uint64(4), bc.BytesSent())
	require.Equal(t, uint64(2), bc.BytesReceived())
}
