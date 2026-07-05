// Package sdp is deprecated. Use pion/sdp directly.
package sdp

import (
	"github.com/pion/sdp/v3"

	"github.com/bluenviron/gortsplib/v5/pkg/sdpunmarshaler"
)

// SessionDescription is a SDP session description.
//
// Deprecated: use pion/sdp directly.
type SessionDescription sdp.SessionDescription

// Attribute returns the value of an attribute and if it exists
//
// Deprecated: use pion/sdp directly.
func (s *SessionDescription) Attribute(key string) (string, bool) {
	return (*sdp.SessionDescription)(s).Attribute(key)
}

// Marshal encodes a SessionDescription.
//
// Deprecated: use pion/sdp directly.
func (s *SessionDescription) Marshal() ([]byte, error) {
	return (*sdp.SessionDescription)(s).Marshal()
}

// Unmarshal decodes a SessionDescription.
//
// Deprecated: use sdpunmarshaler.Unmarshal.
func (s *SessionDescription) Unmarshal(byts []byte) error {
	s2, err := sdpunmarshaler.Unmarshal(byts)
	if err != nil {
		return err
	}

	*s = *(*SessionDescription)(s2)
	return nil
}
