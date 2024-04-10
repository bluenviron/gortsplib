package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPathSplitQuery(t *testing.T) {
	for _, ca := range []struct {
		a string
		b string
		c string
	}{
		{
			"test?a=b",
			"test",
			"a=b",
		},
		{
			"test",
			"test",
			"",
		},
	} {
		b, c := PathSplitQuery(ca.a)
		require.Equal(t, ca.b, b)
		require.Equal(t, ca.c, c)
	}
}
