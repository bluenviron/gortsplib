package liberrors

import (
	"fmt"

	"github.com/aler9/gortsplib/v2/pkg/base"
)

// ErrClientTerminated is an error that can be returned by a client.
type ErrClientTerminated struct{}

// Error implements the error interface.
func (e ErrClientTerminated) Error() string {
	return "terminated"
}

// ErrClientInvalidState is an error that can be returned by a client.
type ErrClientInvalidState struct {
	AllowedList []fmt.Stringer
	State       fmt.Stringer
}

// Error implements the error interface.
func (e ErrClientInvalidState) Error() string {
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

// ErrClientBadStatusCode is an error that can be returned by a client.
type ErrClientBadStatusCode struct {
	Code    base.StatusCode
	Message string
}

// Error implements the error interface.
func (e ErrClientBadStatusCode) Error() string {
	return fmt.Sprintf("bad status code: %d (%s)", e.Code, e.Message)
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

// ErrClientCannotSetupMediasDifferentURLs is an error that can be returned by a client.
type ErrClientCannotSetupMediasDifferentURLs struct{}

// Error implements the error interface.
func (e ErrClientCannotSetupMediasDifferentURLs) Error() string {
	return "cannot setup medias with different base URLs"
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

// ErrClientServerRequestedTCP is an error that can be returned by a client.
type ErrClientServerRequestedTCP struct{}

// Error implements the error interface.
func (e ErrClientServerRequestedTCP) Error() string {
	return "server wants to use the TCP transport protocol"
}

// ErrClientServerRequestedUDP is an error that can be returned by a client.
type ErrClientServerRequestedUDP struct{}

// Error implements the error interface.
func (e ErrClientServerRequestedUDP) Error() string {
	return "server wants to use the UDP transport protocol"
}

// ErrClientTransportHeaderInvalidDelivery is an error that can be returned by a client.
type ErrClientTransportHeaderInvalidDelivery struct{}

// Error implements the error interface.
func (e ErrClientTransportHeaderInvalidDelivery) Error() string {
	return "transport header contains an invalid delivery value"
}

// ErrClientTransportHeaderNoPorts is an error that can be returned by a client.
type ErrClientTransportHeaderNoPorts struct{}

// Error implements the error interface.
func (e ErrClientTransportHeaderNoPorts) Error() string {
	return "transport header does not contain ports"
}

// ErrClientTransportHeaderNoDestination is an error that can be returned by a client.
type ErrClientTransportHeaderNoDestination struct{}

// Error implements the error interface.
func (e ErrClientTransportHeaderNoDestination) Error() string {
	return "transport header does not contain a destination"
}

// ErrClientTransportHeaderNoInterleavedIDs is an error that can be returned by a client.
type ErrClientTransportHeaderNoInterleavedIDs struct{}

// Error implements the error interface.
func (e ErrClientTransportHeaderNoInterleavedIDs) Error() string {
	return "transport header does not contain interleaved IDs"
}

// ErrClientTransportHeaderInvalidInterleavedIDs is an error that can be returned by a client.
type ErrClientTransportHeaderInvalidInterleavedIDs struct{}

// Error implements the error interface.
func (e ErrClientTransportHeaderInvalidInterleavedIDs) Error() string {
	return "invalid interleaved IDs"
}

// ErrClientTransportHeaderInterleavedIDsAlreadyUsed is an error that can be returned by a client.
type ErrClientTransportHeaderInterleavedIDsAlreadyUsed struct{}

// Error implements the error interface.
func (e ErrClientTransportHeaderInterleavedIDsAlreadyUsed) Error() string {
	return "interleaved IDs already used"
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
