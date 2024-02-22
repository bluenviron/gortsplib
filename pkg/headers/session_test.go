package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
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
			Timeout: uintPtr(47),
		},
	},
	{
		"with timeout and space",
		base.HeaderValue{`A3eqwsafq3rFASqew; timeout=47`},
		base.HeaderValue{`A3eqwsafq3rFASqew;timeout=47`},
		Session{
			Session: "A3eqwsafq3rFASqew",
			Timeout: uintPtr(47),
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

func TestSessionMarshal(t *testing.T) {
	for _, ca := range casesSession {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Marshal()
			require.Equal(t, ca.vout, req)
		})
	}
}

func FuzzSessionUnmarshal(f *testing.F) {
	for _, ca := range casesSession {
		f.Add(ca.vin[0])
	}

	f.Add("timeout=")

	f.Fuzz(func(t *testing.T, b string) {
		var h Session
		h.Unmarshal(base.HeaderValue{b}) //nolint:errcheck
	})
}

func TestSessionAdditionalErrors(t *testing.T) {
	func() {
		var h Session
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h Session
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
