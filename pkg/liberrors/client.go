package liberrors

import (
	"fmt"

	"github.com/aler9/gortsplib/pkg/base"
)

// ErrClientWrongState is an error that can be returned by a client.
type ErrClientWrongState struct {
	AllowedList []fmt.Stringer
	State       fmt.Stringer
}

// Error implements the error interface.
func (e ErrClientWrongState) Error() string {
	return fmt.Sprintf("must be in state %v, while is in state %v",
		e.AllowedList, e.State)
}

// ErrClientSessionHeaderInvalid is an error that can be returned by a client.
type ErrClientSessionHeaderInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrClientSessionHeaderInvalid) Error() string {
	return fmt.Sprintf("invalid session header: %v", e.Err)
}

// ErrClientWrongStatusCode is an error that can be returned by a client.
type ErrClientWrongStatusCode struct {
	Code    base.StatusCode
	Message string
}

// Error implements the error interface.
func (e ErrClientWrongStatusCode) Error() string {
	return fmt.Sprintf("wrong status code: %d (%s)", e.Code, e.Message)
}

// ErrClientContentTypeMissing is an error that can be returned by a client.
type ErrClientContentTypeMissing struct{}

// Error implements the error interface.
func (e ErrClientContentTypeMissing) Error() string {
	return "Content-Type header is missing"
}

// ErrClientContentTypeUnsupported is an error that can be returned by a client.
type ErrClientContentTypeUnsupported struct {
	CT base.HeaderValue
}

// Error implements the error interface.
func (e ErrClientContentTypeUnsupported) Error() string {
	return fmt.Sprintf("unsupported Content-Type header '%v'", e.CT)
}

// ErrClientCannotReadPublishAtSameTime is an error that can be returned by a client.
type ErrClientCannotReadPublishAtSameTime struct{}

// Error implements the error interface.
func (e ErrClientCannotReadPublishAtSameTime) Error() string {
	return "cannot read and publish at the same time"
}

// ErrClientCannotSetupTracksDifferentURLs is an error that can be returned by a client.
type ErrClientCannotSetupTracksDifferentURLs struct{}

// Error implements the error interface.
func (e ErrClientCannotSetupTracksDifferentURLs) Error() string {
	return "cannot setup tracks with different base URLs"
}

// ErrClientUDPPortsZero is an error that can be returned by a client.
type ErrClientUDPPortsZero struct{}

// Error implements the error interface.
func (e ErrClientUDPPortsZero) Error() string {
	return "rtpPort and rtcpPort must be both zero or non-zero"
}

// ErrClientUDPPortsNotConsecutive is an error that can be returned by a client.
type ErrClientUDPPortsNotConsecutive struct{}

// Error implements the error interface.
func (e ErrClientUDPPortsNotConsecutive) Error() string {
	return "rtcpPort must be rtpPort + 1"
}

// ErrClientServerPortsZero is an error that can be returned by a client.
type ErrClientServerPortsZero struct{}

// Error implements the error interface.
func (e ErrClientServerPortsZero) Error() string {
	return "server ports must be both zero or both not zero"
}

// ErrClientServerPortsNotProvided is an error that can be returned by a client.
type ErrClientServerPortsNotProvided struct{}

// Error implements the error interface.
func (e ErrClientServerPortsNotProvided) Error() string {
	return "server ports have not been provided. Use AnyPortEnable to communicate with this server"
}

// ErrClientTransportHeaderInvalid is an error that can be returned by a client.
type ErrClientTransportHeaderInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrClientTransportHeaderInvalid) Error() string {
	return fmt.Sprintf("invalid transport header: %v", e.Err)
}

// ErrClientTransportHeaderNoInterleavedIDs is an error that can be returned by a client.
type ErrClientTransportHeaderNoInterleavedIDs struct{}

// Error implements the error interface.
func (e ErrClientTransportHeaderNoInterleavedIDs) Error() string {
	return "transport header does not contain interleaved IDs"
}

// ErrClientTransportHeaderWrongInterleavedIDs is an error that can be returned by a client.
type ErrClientTransportHeaderWrongInterleavedIDs struct {
	Expected [2]int
	Value    [2]int
}

// Error implements the error interface.
func (e ErrClientTransportHeaderWrongInterleavedIDs) Error() string {
	return fmt.Sprintf("wrong interleaved IDs, expected %v, got %v", e.Expected, e.Value)
}

// ErrClientNoUDPPacketsRecently is an error that can be returned by a client.
type ErrClientNoUDPPacketsRecently struct{}

// Error implements the error interface.
func (e ErrClientNoUDPPacketsRecently) Error() string {
	return "no UDP packets received (maybe there's a firewall/NAT in between)"
}

// ErrClientUDPTimeout is an error that can be returned by a client.
type ErrClientUDPTimeout struct{}

// Error implements the error interface.
func (e ErrClientUDPTimeout) Error() string {
	return "UDP timeout"
}

// ErrClientTCPTimeout is an error that can be returned by a client.
type ErrClientTCPTimeout struct{}

// Error implements the error interface.
func (e ErrClientTCPTimeout) Error() string {
	return "TCP timeout"
}

// ErrClientRTPInfoInvalid is an error that can be returned by a client.
type ErrClientRTPInfoInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrClientRTPInfoInvalid) Error() string {
	return fmt.Sprintf("invalid RTP-Info: %v", e.Err)
}
