package headers

import (
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/mikey"
	"github.com/stretchr/testify/require"
)

var casesKeyMgmt = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    KeyMgmt
}{
	{
		"standard",
		base.HeaderValue{`prot=mikey;` +
			`uri="rtsps://127.0.0.1:8322/stream/trackID=0";` +
			`data="AQAFAHojKV4BAACVjCMnAAAAAAsA6/mdTLBeokwKEGwcAuPrxj6/enyb+` +
			`A2+rNcBAAAAFQABAQEBEAIBAQMBCgcBAQgBAQoBAQAAACIAIAAeX8XvOCzIMh0JTOWivWLxEflTUSp1fjj2i8xG7D9DAA=="`},
		base.HeaderValue{`prot=mikey;` +
			`uri="rtsps://127.0.0.1:8322/stream/trackID=0";` +
			`data="AQAFAHojKV4BAACVjCMnAAAAAAsA6/mdTLBeokwKEGwcAuPrxj6/enyb+` +
			`A2+rNcBAAAAFQABAQEBEAIBAQMBCgcBAQgBAQoBAQAAACIAIAAeX8XvOCzIMh0JTOWivWLxEflTUSp1fjj2i8xG7D9DAA=="`},
		KeyMgmt{
			URL: "rtsps://127.0.0.1:8322/stream/trackID=0",
			MikeyMessage: &mikey.Message{
				Header: mikey.Header{
					Version: 1,
					CSBID:   2049124702,
					CSIDMapInfo: []mikey.SRTPIDEntry{
						{
							SSRC: 2508989223,
						},
					},
				},
				Payloads: []mikey.Payload{
					&mikey.PayloadT{
						TSValue: 17003794820816085580,
					},
					&mikey.PayloadRAND{
						Data: []byte{
							0x6c, 0x1c, 0x02, 0xe3, 0xeb, 0xc6, 0x3e, 0xbf,
							0x7a, 0x7c, 0x9b, 0xf8, 0x0d, 0xbe, 0xac, 0xd7,
						},
					},
					&mikey.PayloadSP{
						PolicyParams: []mikey.PayloadSPPolicyParam{
							{
								Type: 0, Value: []byte{1},
							},
							{
								Type: 1, Value: []byte{0x10},
							},
							{
								Type: 2, Value: []byte{1},
							},
							{
								Type: 3, Value: []byte{0x0a},
							},
							{
								Type: 7, Value: []byte{1},
							},
							{
								Type: 8, Value: []byte{1},
							},
							{
								Type: 10, Value: []byte{1},
							},
						},
					},
					&mikey.PayloadKEMAC{
						SubPayloads: []*mikey.SubPayloadKeyData{
							{
								Type: 2,
								KeyData: []byte{
									0x5f, 0xc5, 0xef, 0x38, 0x2c, 0xc8, 0x32, 0x1d,
									0x09, 0x4c, 0xe5, 0xa2, 0xbd, 0x62, 0xf1, 0x11,
									0xf9, 0x53, 0x51, 0x2a, 0x75, 0x7e, 0x38, 0xf6,
									0x8b, 0xcc, 0x46, 0xec, 0x3f, 0x43,
								},
							},
						},
					},
				},
			},
		},
	},
}

func TestKeyMgmtUnmarshal(t *testing.T) {
	for _, ca := range casesKeyMgmt {
		t.Run(ca.name, func(t *testing.T) {
			var h KeyMgmt
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestKeyMgmtMarshal(t *testing.T) {
	for _, ca := range casesKeyMgmt {
		t.Run(ca.name, func(t *testing.T) {
			req, err := ca.h.Marshal()
			require.NoError(t, err)
			require.Equal(t, ca.vout, req)
		})
	}
}

func FuzzKeyMgmtUnmarshal(f *testing.F) {
	for _, ca := range casesKeyMgmt {
		f.Add(ca.vin[0])
	}

	f.Fuzz(func(t *testing.T, b string) {
		var h KeyMgmt
		err := h.Unmarshal(base.HeaderValue{b})
		if err != nil {
			return
		}

		_, err = h.Marshal()
		require.NoError(t, err)
	})
}

func TestKeyMgmtAdditionalErrors(t *testing.T) {
	func() {
		var h KeyMgmt
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h KeyMgmt
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
