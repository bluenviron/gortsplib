package bytecounter_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/internal/bytecounter"
)

func TestByteCounter(t *testing.T) {
	bc := bytecounter.New(bytes.NewBuffer(nil), nil, nil)

	_, err := bc.Write([]byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)

	buf := make([]byte, 2)
	_, err = bc.Read(buf)
	require.NoError(t, err)

	require.Equal(t, uint64(4), bc.BytesSent())
	require.Equal(t, uint64(2), bc.BytesReceived())
}
