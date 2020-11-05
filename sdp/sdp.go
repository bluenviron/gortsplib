// Package sdp contains a SDP encoder/decoder compatible with most RTSP implementations.
package sdp

import (
	psdp "github.com/pion/sdp/v3"
)

// SessionDescription is a SDP session description.
type SessionDescription psdp.SessionDescription

// Marshal encodes a SessionDescription.
func (s *SessionDescription) Marshal() ([]byte, error) {
	return (*psdp.SessionDescription)(s).Marshal()
}
