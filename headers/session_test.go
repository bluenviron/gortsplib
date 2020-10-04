package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/base"
)

var casesSession = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    *Session
}{
	{
		"base",
		base.HeaderValue{`A3eqwsafq3rFASqew`},
		base.HeaderValue{`A3eqwsafq3rFASqew`},
		&Session{
			Session: "A3eqwsafq3rFASqew",
		},
	},
	{
		"with timeout",
		base.HeaderValue{`A3eqwsafq3rFASqew;timeout=47`},
		base.HeaderValue{`A3eqwsafq3rFASqew;timeout=47`},
		&Session{
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
		&Session{
			Session: "A3eqwsafq3rFASqew",
			Timeout: func() *uint {
				v := uint(47)
				return &v
			}(),
		},
	},
}

func TestSessionRead(t *testing.T) {
	for _, c := range casesSession {
		t.Run(c.name, func(t *testing.T) {
			req, err := ReadSession(c.vin)
			require.NoError(t, err)
			require.Equal(t, c.h, req)
		})
	}
}

func TestSessionWrite(t *testing.T) {
	for _, c := range casesSession {
		t.Run(c.name, func(t *testing.T) {
			req := c.h.Write()
			require.Equal(t, c.vout, req)
		})
	}
}
