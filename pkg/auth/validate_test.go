package auth

import (
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
	"github.com/stretchr/testify/require"
)

func FuzzValidate(f *testing.F) {
	f.Add(`Invalid`)
	f.Add(`Digest `)
	f.Add(`Digest realm=123`)
	f.Add(`Digest realm=123,nonce=123`)
	f.Add(`Digest realm=123,nonce=123,username=123`)
	f.Add(`Digest realm=123,nonce=123,username=123,uri=123`)
	f.Add(`Digest realm=123,nonce=123,username=123,uri=123,response=123`)
	f.Add(`Digest realm=123,nonce=abcde,username=123,uri=123,response=123`)

	f.Fuzz(func(t *testing.T, a string) {
		Validate( //nolint:errcheck
			&base.Request{
				Method: base.Describe,
				URL:    nil,
				Header: base.Header{
					"Authorization": base.HeaderValue{a},
				},
			},
			"myuser",
			"mypass",
			nil,
			nil,
			"IPCAM",
			"abcde",
		)
	})
}

func TestValidateAdditionalErrors(t *testing.T) {
	err := Validate(
		&base.Request{
			Method: base.Describe,
			URL:    nil,
			Header: base.Header{
				"Authorization": base.HeaderValue{"Basic bXl1c2VyOm15cGFzcw=="},
			},
		},
		"myuser",
		"mypass",
		nil,
		[]headers.AuthMethod{headers.AuthDigestMD5},
		"IPCAM",
		"abcde",
	)
	require.Error(t, err)
}
