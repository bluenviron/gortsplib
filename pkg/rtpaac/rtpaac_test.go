package rtpaac

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

var cases = []struct {
	name string
	dec  *AUAndTimestamp
	enc  []byte
}{
	{
		"single",
		&AUAndTimestamp{
			Timestamp: 25 * time.Millisecond,
			AU:        bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
		},
		mergeBytes(
			[]byte{
				0x80, 0xe0, 0x44, 0xed, 0x88, 0x77, 0x6b, 0x05,
				0x9d, 0xbb, 0x78, 0x12, 0x00, 0x10, 0x02, 0x00,
			},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 8),
		),
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			sequenceNumber := uint16(0x44ed)
			ssrc := uint32(0x9dbb7812)
			initialTs := uint32(0x88776655)
			e := NewEncoder(96, 48000, &sequenceNumber, &ssrc, &initialTs)
			enc, err := e.Encode(ca.dec)
			require.NoError(t, err)
			require.Equal(t, ca.enc, enc)
		})
	}
}

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := NewDecoder(48000)

			// send an initial packet downstream
			// in order to correctly compute the timestamp
			_, err := d.Decode([]byte{
				0x80, 0xe0, 0x44, 0xed, 0x88, 0x77, 0x66, 0x55,
				0x9d, 0xbb, 0x78, 0x12, 0x00, 0x10, 0x00, 0x08, 0x0,
			})
			require.NoError(t, err)

			dec, err := d.Decode(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}
