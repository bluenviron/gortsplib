package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/v2/pkg/base"
)

var casesSession = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    Session
}{
	{
		"base",
		base.HeaderValue{`A3eqwsafq3rFASqew`},
		base.HeaderValue{`A3eqwsafq3rFASqew`},
		Session{
			Session: "A3eqwsafq3rFASqew",
		},
	},
	{
		"with timeout",
		base.HeaderValue{`A3eqwsafq3rFASqew;timeout=47`},
		base.HeaderValue{`A3eqwsafq3rFASqew;timeout=47`},
		Session{
			Session: "A3eqwsafq3rFASqew",
			Timeout: func() *uint {
				v := uint(47)
				return &v
			}(),
		},
	},
	{
		"with timeout and space",
		base.HeaderValue{`A3eqwsafq3rFASqew; timeout=47`},
		base.HeaderValue{`A3eqwsafq3rFASqew;timeout=47`},
		Session{
			Session: "A3eqwsafq3rFASqew",
			Timeout: func() *uint {
				v := uint(47)
				return &v
			}(),
		},
	},
}

func TestSessionUnmarshal(t *testing.T) {
	for _, ca := range casesSession {
		t.Run(ca.name, func(t *testing.T) {
			var h Session
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestSessionUnmarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		hv   base.HeaderValue
		err  string
	}{
		{
			"empty",
			base.HeaderValue{},
			"value not provided",
		},
		{
			"2 values",
			base.HeaderValue{"a", "b"},
			"value provided multiple times ([a b])",
		},
		{
			"invalid key-value",
			base.HeaderValue{"A3eqwsafq3rFASqew;test=\"a"},
			"apexes not closed (test=\"a)",
		},
		{
			"invalid timeout",
			base.HeaderValue{`A3eqwsafq3rFASqew;timeout=aaa`},
			"strconv.ParseUint: parsing \"aaa\": invalid syntax",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var h Session
			err := h.Unmarshal(ca.hv)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestSessionMarshal(t *testing.T) {
	for _, ca := range casesSession {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Marshal()
			require.Equal(t, ca.vout, req)
		})
	}
}
