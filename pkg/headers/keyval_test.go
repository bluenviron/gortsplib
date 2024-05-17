package headers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesKeyVal = []struct {
	name string
	s    string
	kvs  map[string]string
}{
	{
		"base",
		`key1=v1,key2=v2`,
		map[string]string{
			"key1": "v1",
			"key2": "v2",
		},
	},
	{
		"with space",
		`key1=v1, key2=v2`,
		map[string]string{
			"key1": "v1",
			"key2": "v2",
		},
	},
	{
		"with apexes",
		`key1="v1", key2=v2`,
		map[string]string{
			"key1": "v1",
			"key2": "v2",
		},
	},
	{
		"with apexes and comma",
		`key1="v,1", key2="v2"`,
		map[string]string{
			"key1": "v,1",
			"key2": "v2",
		},
	},
	{
		"with apexes and equal",
		`key1="v=1", key2="v2"`,
		map[string]string{
			"key1": "v=1",
			"key2": "v2",
		},
	},
	{
		"no val key1",
		`key1, key2="v2"`,
		map[string]string{
			"key1": "",
			"key2": "v2",
		},
	},
	{
		"no val key2",
		`key1="v=1", key2`,
		map[string]string{
			"key1": "v=1",
			"key2": "",
		},
	},
	{
		"no val key1 nor key2",
		`key1, key2`,
		map[string]string{
			"key1": "",
			"key2": "",
		},
	},
}

func TestKeyValParse(t *testing.T) {
	for _, ca := range casesKeyVal {
		t.Run(ca.name, func(t *testing.T) {
			kvs, err := keyValParse(ca.s, ',')
			require.NoError(t, err)
			require.Equal(t, ca.kvs, kvs)
		})
	}
}

func FuzzKeyValParse(f *testing.F) {
	for _, ca := range casesKeyVal {
		f.Add(ca.s)
	}

	f.Fuzz(func(_ *testing.T, b string) {
		keyValParse(b, ',') //nolint:errcheck
	})
}
