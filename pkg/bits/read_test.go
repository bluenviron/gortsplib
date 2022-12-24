package bits

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadBits(t *testing.T) {
	buf := []byte{0xA8, 0xC7, 0xD6, 0xAA, 0xBB, 0x10}
	pos := 0
	v, _ := ReadBits(buf, &pos, 6)
	require.Equal(t, uint64(0x2a), v)
	v, _ = ReadBits(buf, &pos, 6)
	require.Equal(t, uint64(0x0c), v)
	v, _ = ReadBits(buf, &pos, 6)
	require.Equal(t, uint64(0x1f), v)
	v, _ = ReadBits(buf, &pos, 8)
	require.Equal(t, uint64(0x5a), v)
	v, _ = ReadBits(buf, &pos, 20)
	require.Equal(t, uint64(0xaaec4), v)
}

func TestReadBitsError(t *testing.T) {
	buf := []byte{0xA8}
	pos := 0
	_, err := ReadBits(buf, &pos, 6)
	require.NoError(t, err)
	_, err = ReadBits(buf, &pos, 6)
	require.EqualError(t, err, "not enough bits")
}

func TestReadGolombUnsigned(t *testing.T) {
	buf := []byte{0x38}
	pos := 0
	v, _ := ReadGolombUnsigned(buf, &pos)
	require.Equal(t, uint32(6), v)
}

func TestReadGolombUnsignedErrors(t *testing.T) {
	buf := []byte{0x00}
	pos := 0
	_, err := ReadGolombUnsigned(buf, &pos)
	require.EqualError(t, err, "not enough bits")

	buf = []byte{0x00, 0x01}
	pos = 0
	_, err = ReadGolombUnsigned(buf, &pos)
	require.EqualError(t, err, "not enough bits")

	buf = []byte{0x00, 0x00, 0x00, 0x00, 0x01}
	pos = 0
	_, err = ReadGolombUnsigned(buf, &pos)
	require.EqualError(t, err, "invalid value")
}

func TestReadGolombSigned(t *testing.T) {
	buf := []byte{0x38}
	pos := 0
	v, _ := ReadGolombSigned(buf, &pos)
	require.Equal(t, int32(-3), v)

	buf = []byte{0b00100100}
	pos = 0
	v, _ = ReadGolombSigned(buf, &pos)
	require.Equal(t, int32(2), v)
}

func TestReadGolombSignedErrors(t *testing.T) {
	buf := []byte{0x00}
	pos := 0
	_, err := ReadGolombSigned(buf, &pos)
	require.EqualError(t, err, "not enough bits")
}

func TestReadFlag(t *testing.T) {
	buf := []byte{0xFF}
	pos := 0
	v, _ := ReadFlag(buf, &pos)
	require.Equal(t, true, v)
}

func TestReadFlagError(t *testing.T) {
	buf := []byte{}
	pos := 0
	_, err := ReadFlag(buf, &pos)
	require.EqualError(t, err, "not enough bits")
}
