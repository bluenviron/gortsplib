package rtph264

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mergeBytes(vals ...[]byte) []byte {
	size := 0
	for _, v := range vals {
		size += len(v)
	}
	res := make([]byte, size)

	pos := 0
	for _, v := range vals {
		n := copy(res[pos:], v)
		pos += n
	}

	return res
}

type readerFunc func(p []byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) {
	return f(p)
}

var cases = []struct {
	name string
	dec  *NALUAndTimestamp
	enc  [][]byte
}{
	{
		"single",
		&NALUAndTimestamp{
			Timestamp: 25 * time.Millisecond,
			NALU: mergeBytes(
				[]byte{0x05},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
			),
		},
		[][]byte{
			mergeBytes(
				[]byte{
					0x80, 0xe0, 0x44, 0xed, 0x88, 0x77, 0x6f, 0x1f,
					0x9d, 0xbb, 0x78, 0x12, 0x05,
				},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
			),
		},
	},
	{
		"fragmented",
		&NALUAndTimestamp{
			Timestamp: 55 * time.Millisecond,
			NALU: mergeBytes(
				[]byte{0x05},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 256),
			),
		},
		[][]byte{
			mergeBytes(
				[]byte{
					0x80, 0x60, 0x44, 0xed, 0x88, 0x77, 0x79, 0xab,
					0x9d, 0xbb, 0x78, 0x12, 0x1c, 0x85, 0x00, 0x01,
					0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
				},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 181),
				[]byte{0x00, 0x01},
			),
			mergeBytes(
				[]byte{
					0x80, 0xe0, 0x44, 0xee, 0x88, 0x77, 0x79, 0xab,
					0x9d, 0xbb, 0x78, 0x12, 0x1c, 0x45, 0x02, 0x03,
					0x04, 0x05, 0x06, 0x07,
				},
				bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 73),
			),
		},
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			sequenceNumber := uint16(0x44ed)
			ssrc := uint32(0x9dbb7812)
			initialTs := uint32(0x88776655)
			e := NewEncoder(96, &sequenceNumber, &ssrc, &initialTs)
			enc, err := e.Encode(ca.dec)
			require.NoError(t, err)
			require.Equal(t, ca.enc, enc)
		})
	}
}

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			i := 0
			r := readerFunc(func(p []byte) (int, error) {
				if i == 0 {
					// send an initial packet downstream
					// in order to correctly compute the timestamp
					n := copy(p, []byte{
						0x80, 0xe0, 0x44, 0xed, 0x88, 0x77, 0x66, 0x55,
						0x9d, 0xbb, 0x78, 0x12, 0x06, 0x00,
					})
					i++
					return n, nil
				}

				n := copy(p, ca.enc[i-1])
				i++
				return n, nil
			})

			d := NewDecoder()

			_, err := d.Read(r)
			require.NoError(t, err)

			dec, err := d.Read(r)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}
