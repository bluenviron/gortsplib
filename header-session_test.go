package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesHeaderSession = []struct {
	name string
	byts string
	hs   *HeaderSession
}{
	{
		"base",
		`A3eqwsafq3rFASqew`,
		&HeaderSession{
			Session: "A3eqwsafq3rFASqew",
		},
	},
	{
		"with timeout",
		`A3eqwsafq3rFASqew;timeout=47`,
		&HeaderSession{
			Session: "A3eqwsafq3rFASqew",
			Timeout: func() *uint {
				v := uint(47)
				return &v
			}(),
		},
	},
	{
		"with timeout and space",
		`A3eqwsafq3rFASqew; timeout=47`,
		&HeaderSession{
			Session: "A3eqwsafq3rFASqew",
			Timeout: func() *uint {
				v := uint(47)
				return &v
			}(),
		},
	},
}

func TestHeaderSession(t *testing.T) {
	for _, c := range casesHeaderSession {
		t.Run(c.name, func(t *testing.T) {
			req, err := ReadHeaderSession(c.byts)
			require.NoError(t, err)
			require.Equal(t, c.hs, req)
		})
	}
}
