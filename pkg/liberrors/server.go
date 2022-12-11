package liberrors

import (
	"fmt"
	"net"

	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/headers"
)

// ErrServerTerminated is an error that can be returned by a server.
type ErrServerTerminated struct{}

// Error implements the error interface.
func (e ErrServerTerminated) Error() string {
	return "terminated"
}

// ErrServerSessionNotFound is an error that can be returned by a server.
type ErrServerSessionNotFound struct{}

// Error implements the error interface.
func (e ErrServerSessionNotFound) Error() string {
	return "session not found"
}

// ErrServerNoUDPPacketsInAWhile is an error that can be returned by a server.
type ErrServerNoUDPPacketsInAWhile struct{}

// Error implements the error interface.
func (e ErrServerNoUDPPacketsInAWhile) Error() string {
	return "no UDP packets received in a while"
}

// ErrServerNoRTSPRequestsInAWhile is an error that can be returned by a server.
type ErrServerNoRTSPRequestsInAWhile struct{}

// Error implements the error interface.
func (e ErrServerNoRTSPRequestsInAWhile) Error() string {
	return "no RTSP requests received in a while"
}

// ErrServerCSeqMissing is an error that can be returned by a server.
type ErrServerCSeqMissing struct{}

// Error implements the error interface.
func (e ErrServerCSeqMissing) Error() string {
	return "CSeq is missing"
}

// ErrServerInvalidState is an error that can be returned by a server.
type ErrServerInvalidState struct {
	AllowedList []fmt.Stringer
	State       fmt.Stringer
}

// Error implements the error interface.
func (e ErrServerInvalidState) Error() string {
	return fmt.Sprintf("must be in state %v, while is in state %v",
		e.AllowedList, e.State)
}

// ErrServerInvalidPath is an error that can be returned by a server.
type ErrServerInvalidPath struct{}

// Error implements the error interface.
func (e ErrServerInvalidPath) Error() string {
	return "invalid path"
}

// ErrServerContentTypeMissing is an error that can be returned by a server.
type ErrServerContentTypeMissing struct{}

// Error implements the error interface.
func (e ErrServerContentTypeMissing) Error() string {
	return "Content-Type header is missing"
}

// ErrServerContentTypeUnsupported is an error that can be returned by a server.
type ErrServerContentTypeUnsupported struct {
	CT base.HeaderValue
}

// Error implements the error interface.
func (e ErrServerContentTypeUnsupported) Error() string {
	return fmt.Sprintf("unsupported Content-Type header '%v'", e.CT)
}

// ErrServerSDPInvalid is an error that can be returned by a server.
type ErrServerSDPInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrServerSDPInvalid) Error() string {
	return fmt.Sprintf("invalid SDP: %v", e.Err)
}

// ErrServerTransportHeaderInvalid is an error that can be returned by a server.
type ErrServerTransportHeaderInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrServerTransportHeaderInvalid) Error() string {
	return fmt.Sprintf("invalid transport header: %v", e.Err)
}

// ErrServerMediaAlreadySetup is an error that can be returned by a server.
type ErrServerMediaAlreadySetup struct{}

// Error implements the error interface.
func (e ErrServerMediaAlreadySetup) Error() string {
	return "media has already been setup"
}

// ErrServerTransportHeaderInvalidMode is an error that can be returned by a server.
type ErrServerTransportHeaderInvalidMode struct {
	Mode headers.TransportMode
}

// Error implements the error interface.
func (e ErrServerTransportHeaderInvalidMode) Error() string {
	return fmt.Sprintf("transport header contains a invalid mode (%v)", e.Mode)
}

// ErrServerTransportHeaderNoClientPorts is an error that can be returned by a server.
type ErrServerTransportHeaderNoClientPorts struct{}

// Error implements the error interface.
func (e ErrServerTransportHeaderNoClientPorts) Error() string {
	return "transport header does not contain client ports"
}

// ErrServerTransportHeaderNoInterleavedIDs is an error that can be returned by a server.
type ErrServerTransportHeaderNoInterleavedIDs struct{}

// Error implements the error interface.
func (e ErrServerTransportHeaderNoInterleavedIDs) Error() string {
	return "transport header does not contain interleaved IDs"
}

// ErrServerTransportHeaderInvalidInterleavedIDs is an error that can be returned by a server.
type ErrServerTransportHeaderInvalidInterleavedIDs struct{}

// Error implements the error interface.
func (e ErrServerTransportHeaderInvalidInterleavedIDs) Error() string {
	return "invalid interleaved IDs"
}

// ErrServerTransportHeaderInterleavedIDsAlreadyUsed is an error that can be returned by a server.
type ErrServerTransportHeaderInterleavedIDsAlreadyUsed struct{}

// Error implements the error interface.
func (e ErrServerTransportHeaderInterleavedIDsAlreadyUsed) Error() string {
	return "interleaved IDs already used"
}

// ErrServerMediasDifferentPaths is an error that can be returned by a server.
type ErrServerMediasDifferentPaths struct{}

// Error implements the error interface.
func (e ErrServerMediasDifferentPaths) Error() string {
	return "can't setup medias with different paths"
}

// ErrServerMediasDifferentProtocols is an error that can be returned by a server.
type ErrServerMediasDifferentProtocols struct{}

// Error implements the error interface.
func (e ErrServerMediasDifferentProtocols) Error() string {
	return "can't setup medias with different protocols"
}

// ErrServerNoMediasSetup is an error that can be returned by a server.
type ErrServerNoMediasSetup struct{}

// Error implements the error interface.
func (e ErrServerNoMediasSetup) Error() string {
	return "no medias have been setup"
}

// ErrServerNotAllAnnouncedMediasSetup is an error that can be returned by a server.
type ErrServerNotAllAnnouncedMediasSetup struct{}

// Error implements the error interface.
func (e ErrServerNotAllAnnouncedMediasSetup) Error() string {
	return "not all announced medias have been setup"
}

// ErrServerLinkedToOtherSession is an error that can be returned by a server.
type ErrServerLinkedToOtherSession struct{}

// Error implements the error interface.
func (e ErrServerLinkedToOtherSession) Error() string {
	return "connection is linked to another session"
}

// ErrServerSessionTeardown is an error that can be returned by a server.
type ErrServerSessionTeardown struct {
	Author net.Addr
}

// Error implements the error interface.
func (e ErrServerSessionTeardown) Error() string {
	return fmt.Sprintf("teared down by %v", e.Author)
}

// ErrServerSessionLinkedToOtherConn is an error that can be returned by a server.
type ErrServerSessionLinkedToOtherConn struct{}

// Error implements the error interface.
func (e ErrServerSessionLinkedToOtherConn) Error() string {
	return "session is linked to another connection"
}

// ErrServerInvalidSession is an error that can be returned by a server.
type ErrServerInvalidSession struct{}

// Error implements the error interface.
func (e ErrServerInvalidSession) Error() string {
	return "invalid session"
}

// ErrServerPathHasChanged is an error that can be returned by a server.
type ErrServerPathHasChanged struct {
	Prev string
	Cur  string
}

// Error implements the error interface.
func (e ErrServerPathHasChanged) Error() string {
	return fmt.Sprintf("path has changed, was '%s', now is '%s'", e.Prev, e.Cur)
}

// ErrServerCannotUseSessionCreatedByOtherIP is an error that can be returned by a server.
type ErrServerCannotUseSessionCreatedByOtherIP struct{}

// Error implements the error interface.
func (e ErrServerCannotUseSessionCreatedByOtherIP) Error() string {
	return "cannot use a session created with a different IP"
}

// ErrServerUDPPortsAlreadyInUse is an error that can be returned by a server.
type ErrServerUDPPortsAlreadyInUse struct {
	Port int
}

// Error implements the error interface.
func (e ErrServerUDPPortsAlreadyInUse) Error() string {
	return fmt.Sprintf("UDP ports %d and %d are already assigned to another reader with the same IP",
		e.Port, e.Port+1)
}

// ErrServerSessionNotInUse is an error that can be returned by a server.
type ErrServerSessionNotInUse struct{}

// Error implements the error interface.
func (e ErrServerSessionNotInUse) Error() string {
	return "not in use"
}

// ErrServerUnexpectedFrame is an error that can be returned by a server.
type ErrServerUnexpectedFrame struct{}

// Error implements the error interface.
func (e ErrServerUnexpectedFrame) Error() string {
	return "received unexpected interleaved frame"
}
