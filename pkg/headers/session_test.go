package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
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

func TestSessionRead(t *testing.T) {
	for _, ca := range casesSession {
		t.Run(ca.name, func(t *testing.T) {
			var h Session
			err := h.Read(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestSessionReadError(t *testing.T) {
	for _, ca := range []struct {
		name string
		hv   base.HeaderValue
	}{
		{
			"empty",
			base.HeaderValue{},
		},
		{
			"2 values",
			base.HeaderValue{"a", "b"},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var h Session
			err := h.Read(ca.hv)
			require.Error(t, err)
		})
	}
}

func TestSessionWrite(t *testing.T) {
	for _, ca := range casesSession {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Write()
			require.Equal(t, ca.vout, req)
		})
	}
}
