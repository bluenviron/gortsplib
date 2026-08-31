package sdes_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/sdes"
)

func TestUnmarshal(t *testing.T) {
	// this exact inline key was captured from a real UniFi Protect camera's
	// RTSPS ("?enableSrtp") DESCRIBE response.
	const inlineKey = "5yQlV6XBpXYqEhsRoCs2OH+GujtAjltr3K6GPpyY"

	expectedKey, err := base64.StdEncoding.DecodeString(inlineKey)
	require.NoError(t, err)
	require.Len(t, expectedKey, 30) // 16-byte master key + 14-byte master salt

	var s sdes.SDES
	err = s.Unmarshal("1 AES_CM_128_HMAC_SHA1_80 inline:" + inlineKey)
	require.NoError(t, err)

	require.Equal(t, sdes.SDES{
		Tag:   1,
		Suite: "AES_CM_128_HMAC_SHA1_80",
		Key:   expectedKey,
	}, s)
}

func TestUnmarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		v    string
		err  string
	}{
		{
			"missing fields",
			"1 AES_CM_128_HMAC_SHA1_80",
			"invalid crypto attribute: 1 AES_CM_128_HMAC_SHA1_80",
		},
		{
			"non-inline key method",
			"1 AES_CM_128_HMAC_SHA1_80 mikey:AQAFgM0=",
			"unsupported crypto key method: mikey:AQAFgM0=",
		},
		{
			"invalid base64",
			"1 AES_CM_128_HMAC_SHA1_80 inline:not-valid-base64!!",
			"invalid crypto inline key: illegal base64 data at input byte 3",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var s sdes.SDES
			err := s.Unmarshal(ca.v)
			require.EqualError(t, err, ca.err)
		})
	}
}
