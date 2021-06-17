package liberrors

import (
	"fmt"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
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

// ErrServerSessionTimedOut is an error that can be returned by a server.
type ErrServerSessionTimedOut struct{}

// Error implements the error interface.
func (e ErrServerSessionTimedOut) Error() string {
	return "timed out"
}

// ErrServerTCPFramesEnable is an error that can be returned by a server.
type ErrServerTCPFramesEnable struct{}

// Error implements the error interface.
func (e ErrServerTCPFramesEnable) Error() string {
	return "tcp frame enable"
}

// ErrServerTCPFramesDisable is an error that can be returned by a server.
type ErrServerTCPFramesDisable struct{}

// Error implements the error interface.
func (e ErrServerTCPFramesDisable) Error() string {
	return "tcp frame disable"
}

// ErrServerCSeqMissing is an error that can be returned by a server.
type ErrServerCSeqMissing struct{}

// Error implements the error interface.
func (e ErrServerCSeqMissing) Error() string {
	return "CSeq is missing"
}

// ErrServerUnhandledRequest is an error that can be returned by a server.
type ErrServerUnhandledRequest struct {
	Req *base.Request
}

// Error implements the error interface.
func (e ErrServerUnhandledRequest) Error() string {
	return fmt.Sprintf("unhandled request (%v %v)", e.Req.Method, e.Req.URL)
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

// ErrServerSDPNoTracksDefined is an error that can be returned by a server.
type ErrServerSDPNoTracksDefined struct{}

// Error implements the error interface.
func (e ErrServerSDPNoTracksDefined) Error() string {
	return "no tracks defined in the SDP"
}

// ErrServerTransportHeaderInvalid is an error that can be returned by a server.
type ErrServerTransportHeaderInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrServerTransportHeaderInvalid) Error() string {
	return fmt.Sprintf("invalid transport header: %v", e.Err)
}

// ErrServerTrackAlreadySetup is an error that can be returned by a server.
type ErrServerTrackAlreadySetup struct {
	TrackID int
}

// Error implements the error interface.
func (e ErrServerTrackAlreadySetup) Error() string {
	return fmt.Sprintf("track %d has already been setup", e.TrackID)
}

// ErrServerTransportHeaderInvalidMode is an error that can be returned by a server.
type ErrServerTransportHeaderInvalidMode struct {
	Mode *headers.TransportMode
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
type ErrServerTransportHeaderInvalidInterleavedIDs struct {
	Expected [2]int
	Value    [2]int
}

// Error implements the error interface.
func (e ErrServerTransportHeaderInvalidInterleavedIDs) Error() string {
	return fmt.Sprintf("invalid interleaved IDs, expected %v, got %v", e.Expected, e.Value)
}

// ErrServerTracksDifferentProtocols is an error that can be returned by a server.
type ErrServerTracksDifferentProtocols struct{}

// Error implements the error interface.
func (e ErrServerTracksDifferentProtocols) Error() string {
	return "can't setup tracks with different protocols"
}

// ErrServerNoTracksSetup is an error that can be returned by a server.
type ErrServerNoTracksSetup struct{}

// Error implements the error interface.
func (e ErrServerNoTracksSetup) Error() string {
	return "no tracks have been setup"
}

// ErrServerNotAllAnnouncedTracksSetup is an error that can be returned by a server.
type ErrServerNotAllAnnouncedTracksSetup struct{}

// Error implements the error interface.
func (e ErrServerNotAllAnnouncedTracksSetup) Error() string {
	return "not all announced tracks have been setup"
}

// ErrServerNoUDPPacketsRecently is an error that can be returned by a server.
type ErrServerNoUDPPacketsRecently struct{}

// Error implements the error interface.
func (e ErrServerNoUDPPacketsRecently) Error() string {
	return "no UDP packets received (maybe there's a firewall/NAT in between)"
}

// ErrServerLinkedToOtherSession is an error that can be returned by a server.
type ErrServerLinkedToOtherSession struct{}

// Error implements the error interface.
func (e ErrServerLinkedToOtherSession) Error() string {
	return "connection is linked to another session"
}

// ErrServerSessionTeardown is an error that can be returned by a server.
type ErrServerSessionTeardown struct{}

// Error implements the error interface.
func (e ErrServerSessionTeardown) Error() string {
	return "teardown"
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
