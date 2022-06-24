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

func TestReadGolombSigned(t *testing.T) {
	buf := []byte{0x38}
	pos := 0
	v, _ := ReadGolombSigned(buf, &pos)
	require.Equal(t, int32(-3), v)
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

func TestUint(t *testing.T) {
	buf := []byte{0x45, 0x46, 0x47, 0x48, 0x49, 0x50, 0x51}
	pos := 0
	u8, _ := ReadUint8(buf, &pos)
	require.Equal(t, uint8(0x45), u8)
	u16, _ := ReadUint16(buf, &pos)
	require.Equal(t, uint16(0x4647), u16)
	u32, _ := ReadUint32(buf, &pos)
	require.Equal(t, uint32(0x48495051), u32)
}
