// Package bits contains functions to read/write bits from/to buffers.
package bits

import (
	"fmt"
)

// ReadBits reads N bits.
func ReadBits(buf []byte, pos *int, n int) (uint64, error) {
	if n > ((len(buf) * 8) - *pos) {
		return 0, fmt.Errorf("not enough bits")
	}

	v := uint64(0)

	res := 8 - (*pos & 0x07)
	if n < res {
		v := uint64((buf[*pos>>0x03] >> (res - n)) & (1<<n - 1))
		*pos += n
		return v, nil
	}

	v = (v << res) | uint64(buf[*pos>>0x03]&(1<<res-1))
	*pos += res
	n -= res

	for n >= 8 {
		v = (v << 8) | uint64(buf[*pos>>0x03])
		*pos += 8
		n -= 8
	}

	if n > 0 {
		v = (v << n) | uint64(buf[*pos>>0x03]>>(8-n))
		*pos += n
	}

	return v, nil
}

// ReadGolombUnsigned reads an unsigned golomb-encoded value.
func ReadGolombUnsigned(buf []byte, pos *int) (uint32, error) {
	buflen := len(buf)
	leadingZeroBits := uint32(0)

	for {
		if (buflen*8 - *pos) == 0 {
			return 0, fmt.Errorf("not enough bits")
		}

		b := (buf[*pos>>0x03] >> (7 - (*pos & 0x07))) & 0x01
		*pos++
		if b != 0 {
			break
		}

		leadingZeroBits++
		if leadingZeroBits > 32 {
			return 0, fmt.Errorf("invalid value")
		}
	}

	if (buflen*8 - *pos) < int(leadingZeroBits) {
		return 0, fmt.Errorf("not enough bits")
	}

	codeNum := uint32(0)

	for n := leadingZeroBits; n > 0; n-- {
		b := (buf[*pos>>0x03] >> (7 - (*pos & 0x07))) & 0x01
		*pos++
		codeNum |= uint32(b) << (n - 1)
	}

	codeNum = (1 << leadingZeroBits) - 1 + codeNum

	return codeNum, nil
}

// ReadGolombSigned reads a signed golomb-encoded value.
func ReadGolombSigned(buf []byte, pos *int) (int32, error) {
	v, err := ReadGolombUnsigned(buf, pos)
	if err != nil {
		return 0, err
	}

	vi := int32(v)
	if (vi & 0x01) != 0 {
		return (vi + 1) / 2, nil
	}
	return -vi / 2, nil
}

// ReadFlag reads a boolean flag.
func ReadFlag(buf []byte, pos *int) (bool, error) {
	if (len(buf)*8 - *pos) == 0 {
		return false, fmt.Errorf("not enough bits")
	}

	b := (buf[*pos>>0x03] >> (7 - (*pos & 0x07))) & 0x01
	*pos++
	return b == 1, nil
}

// ReadUint8 reads a uint8.
func ReadUint8(buf []byte, pos *int) (uint8, error) {
	v, err := ReadBits(buf, pos, 8)
	return uint8(v), err
}

// ReadUint16 reads a uint16.
func ReadUint16(buf []byte, pos *int) (uint16, error) {
	v, err := ReadBits(buf, pos, 16)
	return uint16(v), err
}

// ReadUint32 reads a uint32.
func ReadUint32(buf []byte, pos *int) (uint32, error) {
	v, err := ReadBits(buf, pos, 32)
	return uint32(v), err
}
