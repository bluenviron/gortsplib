package liberrors

import (
	"fmt"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// ErrServerTeardown is returned in case of a teardown request.
type ErrServerTeardown struct{}

// Error implements the error interface.
func (e ErrServerTeardown) Error() string {
	return "teardown"
}

// ErrServerCSeqMissing is returned in case the CSeq is missing.
type ErrServerCSeqMissing struct{}

// Error implements the error interface.
func (e ErrServerCSeqMissing) Error() string {
	return "CSeq is missing"
}

// ErrServerWrongState is returned in case of a wrong client state.
type ErrServerWrongState struct {
	AllowedList []fmt.Stringer
	State       fmt.Stringer
}

// Error implements the error interface.
func (e ErrServerWrongState) Error() string {
	return fmt.Sprintf("must be in state %v, while is in state %v",
		e.AllowedList, e.State)
}

// ErrServerNoPath is returned in case the path can't be retrieved.
type ErrServerNoPath struct{}

// Error implements the error interface.
func (e ErrServerNoPath) Error() string {
	return "RTSP path can't be retrieved"
}

// ErrServerContentTypeMissing is returned in case the Content-Type header is missing.
type ErrServerContentTypeMissing struct{}

// Error implements the error interface.
func (e ErrServerContentTypeMissing) Error() string {
	return "Content-Type header is missing"
}

// ErrServerContentTypeUnsupported is returned in case the Content-Type header is unsupported.
type ErrServerContentTypeUnsupported struct {
	CT base.HeaderValue
}

// Error implements the error interface.
func (e ErrServerContentTypeUnsupported) Error() string {
	return fmt.Sprintf("unsupported Content-Type header '%v'", e.CT)
}

// ErrServerSDPInvalid is returned in case the SDP is invalid.
type ErrServerSDPInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrServerSDPInvalid) Error() string {
	return fmt.Sprintf("invalid SDP: %v", e.Err)
}

// ErrServerSDPNoTracksDefined is returned in case the SDP has no tracks defined.
type ErrServerSDPNoTracksDefined struct{}

// Error implements the error interface.
func (e ErrServerSDPNoTracksDefined) Error() string {
	return "no tracks defined in the SDP"
}

// ErrServerTransportHeaderInvalid is returned in case the transport header is invalid.
type ErrServerTransportHeaderInvalid struct {
	Err error
}

// Error implements the error interface.
func (e ErrServerTransportHeaderInvalid) Error() string {
	return fmt.Sprintf("invalid transport header: %v", e.Err)
}

// ErrServerTrackAlreadySetup is returned in case a track has already been setup.
type ErrServerTrackAlreadySetup struct {
	TrackID int
}

// Error implements the error interface.
func (e ErrServerTrackAlreadySetup) Error() string {
	return fmt.Sprintf("track %d has already been setup", e.TrackID)
}

// ErrServerTransportHeaderWrongMode is returned in case the transport header contains a wrong mode.
type ErrServerTransportHeaderWrongMode struct {
	Mode *headers.TransportMode
}

// Error implements the error interface.
func (e ErrServerTransportHeaderWrongMode) Error() string {
	return fmt.Sprintf("transport header contains a wrong mode (%v)", e.Mode)
}

// ErrServerTransportHeaderNoClientPorts is returned in case the transport header doesn't contain client ports.
type ErrServerTransportHeaderNoClientPorts struct{}

// Error implements the error interface.
func (e ErrServerTransportHeaderNoClientPorts) Error() string {
	return "transport header does not contain client ports"
}

// ErrServerTransportHeaderNoInterleavedIDs is returned in case the transport header doesn't contain interleaved IDs.
type ErrServerTransportHeaderNoInterleavedIDs struct{}

// Error implements the error interface.
func (e ErrServerTransportHeaderNoInterleavedIDs) Error() string {
	return "transport header does not contain interleaved IDs"
}

// ErrServerTransportHeaderWrongInterleavedIDs is returned in case the transport header contains wrong interleaved IDs.
type ErrServerTransportHeaderWrongInterleavedIDs struct {
	Expected [2]int
	Value    [2]int
}

// Error implements the error interface.
func (e ErrServerTransportHeaderWrongInterleavedIDs) Error() string {
	return fmt.Sprintf("wrong interleaved IDs, expected %v, got %v", e.Expected, e.Value)
}

// ErrServerTracksDifferentProtocols is returned in case the client is trying to setup tracks with different protocols.
type ErrServerTracksDifferentProtocols struct{}

// Error implements the error interface.
func (e ErrServerTracksDifferentProtocols) Error() string {
	return "can't setup tracks with different protocols"
}

// ErrServerNoTracksSetup is returned in case no tracks have been setup.
type ErrServerNoTracksSetup struct{}

// Error implements the error interface.
func (e ErrServerNoTracksSetup) Error() string {
	return "no tracks have been setup"
}

// ErrServerNotAllAnnouncedTracksSetup is returned in case not all announced tracks have been setup.
type ErrServerNotAllAnnouncedTracksSetup struct{}

// Error implements the error interface.
func (e ErrServerNotAllAnnouncedTracksSetup) Error() string {
	return "not all announced tracks have been setup"
}

// ErrServerNoUDPPacketsRecently is returned when no UDP packets have been received recently.
type ErrServerNoUDPPacketsRecently struct{}

// Error implements the error interface.
func (e ErrServerNoUDPPacketsRecently) Error() string {
	return "no UDP packets received (maybe there's a firewall/NAT in between)"
}
