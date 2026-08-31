// Package sdes contains a SDES (RFC 4568) key-exchange decoder.
package sdes

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

// SDES holds a parsed SDES (RFC 4568) "a=crypto" media attribute.
//
// This is a separate, independent key-exchange mechanism from MIKEY
// (RFC 3830, see the mikey package): SDES embeds the key material directly,
// inline, in the SDP itself, rather than negotiating it via a MIKEY
// message. It's what UniFi Protect cameras use for their "?enableSrtp"
// RTSPS streams.
type SDES struct {
	// Tag is the crypto suite tag from the SDP line (used to correlate a
	// crypto line with a SETUP response in more elaborate negotiations;
	// not security-relevant on its own).
	Tag int

	// Suite is the crypto suite name, e.g. "AES_CM_128_HMAC_SHA1_80".
	Suite string

	// Key is the raw, still-encoded-for-policy master key + master salt,
	// decoded from the "inline" key parameter. Its expected length and
	// interpretation depend on Suite.
	Key []byte
}

// Unmarshal decodes the value of an SDP "a=crypto" attribute, e.g.:
//
//	1 AES_CM_128_HMAC_SHA1_80 inline:5yQlV6XBpXYqEhsRoCs2OH+GujtAjltr3K6GPpyY
//
// Per RFC 4568, the general form is:
//
//	<tag> <crypto-suite> <key-params> [<session-params>]
//
// where <key-params> for inline keys is:
//
//	inline:<base64 key||salt>[|<lifetime>][|<mki>:<length>]
//
// This only extracts the tag, suite name, and decoded key material — it
// deliberately does not validate the suite or key length here, matching how
// the "key-mgmt" (MIKEY) attribute is handled: structural decoding happens
// during SDP unmarshal, and policy/suite validation happens later, when the
// key is turned into a SRTP context.
func (s *SDES) Unmarshal(v string) error {
	fields := strings.Fields(v)
	if len(fields) < 3 {
		return fmt.Errorf("invalid crypto attribute: %v", v)
	}

	tag, err := strconv.Atoi(fields[0])
	if err != nil {
		return fmt.Errorf("invalid crypto tag: %v", fields[0])
	}

	suite := fields[1]

	const inlinePrefix = "inline:"
	if !strings.HasPrefix(fields[2], inlinePrefix) {
		return fmt.Errorf("unsupported crypto key method: %v", fields[2])
	}

	// the key parameter is <base64>[|<lifetime>][|<mki>:<length>];
	// only the base64 key material is needed to decrypt.
	rawKey := strings.TrimPrefix(fields[2], inlinePrefix)
	rawKey, _, _ = strings.Cut(rawKey, "|")

	key, err := base64.StdEncoding.DecodeString(rawKey)
	if err != nil {
		return fmt.Errorf("invalid crypto inline key: %w", err)
	}

	s.Tag = tag
	s.Suite = suite
	s.Key = key

	return nil
}
