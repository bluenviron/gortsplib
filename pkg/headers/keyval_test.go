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
		"with apices",
		`key1="v1", key2=v2`,
		map[string]string{
			"key1": "v1",
			"key2": "v2",
		},
	},
	{
		"with apices and comma",
		`key1="v,1", key2="v2"`,
		map[string]string{
			"key1": "v,1",
			"key2": "v2",
		},
	},
	{
		"with apices and equal",
		`key1="v=1", key2="v2"`,
		map[string]string{
			"key1": "v=1",
			"key2": "v2",
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
