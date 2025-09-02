package ntp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var cases = []struct {
	name string
	dec  time.Time
	enc  uint64
}{
	{
		"a",
		time.Date(2013, 4, 15, 11, 15, 17, 958404853, time.UTC).Local(),
		15354565283395798332,
	},
	{
		"b",
		time.Date(2013, 4, 15, 11, 15, 18, 0, time.UTC).Local(),
		15354565283574448128,
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			v := Encode(ca.dec)
			require.Equal(t, ca.enc, v)
		})
	}
}

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			v := Decode(ca.enc)
			require.Equal(t, ca.dec, v)
		})
	}
}
