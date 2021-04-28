package gortsplib

import (
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// ServerHandler is the interface implemented by all the server handlers.
type ServerHandler interface {
}

// ServerHandlerOnConnOpen can be implemented by a ServerHandler.
type ServerHandlerOnConnOpen interface {
	OnConnOpen(*ServerConn)
}

// ServerHandlerOnConnClose can be implemented by a ServerHandler.
type ServerHandlerOnConnClose interface {
	OnConnClose(*ServerConn, error)
}

// ServerHandlerOnRequest can be implemented by a ServerHandler.
type ServerHandlerOnRequest interface {
	OnRequest(*base.Request)
}

// ServerHandlerOnResponse can be implemented by a ServerHandler.
type ServerHandlerOnResponse interface {
	OnResponse(*base.Response)
}

// ServerHandlerOnOptionsCtx is the context of an OPTIONS request.
type ServerHandlerOnOptionsCtx struct {
	Conn  *ServerConn
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnOptions can be implemented by a ServerHandler.
type ServerHandlerOnOptions interface {
	OnOptions(*ServerHandlerOnOptionsCtx) (*base.Response, error)
}

// ServerHandlerOnDescribeCtx is the context of a DESCRIBE request.
type ServerHandlerOnDescribeCtx struct {
	Conn  *ServerConn
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnDescribe can be implemented by a ServerHandler.
type ServerHandlerOnDescribe interface {
	OnDescribe(*ServerHandlerOnDescribeCtx) (*base.Response, []byte, error)
}

// ServerHandlerOnAnnounceCtx is the context of an ANNOUNCE request.
type ServerHandlerOnAnnounceCtx struct {
	Conn *ServerConn
	// Session *ServerSession
	Req    *base.Request
	Path   string
	Query  string
	Tracks Tracks
}

// ServerHandlerOnAnnounce can be implemented by a ServerHandler.
type ServerHandlerOnAnnounce interface {
	OnAnnounce(*ServerHandlerOnAnnounceCtx) (*base.Response, error)
}

// ServerHandlerOnSetupCtx is the context of a OPTIONS request.
type ServerHandlerOnSetupCtx struct {
	Conn      *ServerConn
	Session   *ServerSession
	Req       *base.Request
	Path      string
	Query     string
	TrackID   int
	Transport *headers.Transport
}

// ServerHandlerOnSetup can be implemented by a ServerHandler.
type ServerHandlerOnSetup interface {
	OnSetup(*ServerHandlerOnSetupCtx) (*base.Response, error)
}

// ServerHandlerOnPlayCtx is the context of a PLAY request.
type ServerHandlerOnPlayCtx struct {
	Conn *ServerConn
	// Session *ServerSession
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnPlay can be implemented by a ServerHandler.
type ServerHandlerOnPlay interface {
	OnPlay(*ServerHandlerOnPlayCtx) (*base.Response, error)
}

// ServerHandlerOnRecordCtx is the context of a RECORD request.
type ServerHandlerOnRecordCtx struct {
	Conn *ServerConn
	// Session *ServerSession
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnRecord can be implemented by a ServerHandler.
type ServerHandlerOnRecord interface {
	OnRecord(*ServerHandlerOnRecordCtx) (*base.Response, error)
}

// ServerHandlerOnPauseCtx is the context of a PAUSE request.
type ServerHandlerOnPauseCtx struct {
	Conn *ServerConn
	// Session *ServerSession
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnPause can be implemented by a ServerHandler.
type ServerHandlerOnPause interface {
	OnPause(*ServerHandlerOnPauseCtx) (*base.Response, error)
}

// ServerHandlerOnGetParameterCtx is the context of a GET_PARAMETER request.
type ServerHandlerOnGetParameterCtx struct {
	Conn  *ServerConn
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnGetParameter can be implemented by a ServerHandler.
type ServerHandlerOnGetParameter interface {
	OnGetParameter(*ServerHandlerOnGetParameterCtx) (*base.Response, error)
}

// ServerHandlerOnSetParameterCtx is the context of a SET_PARAMETER request.
type ServerHandlerOnSetParameterCtx struct {
	Conn  *ServerConn
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnSetParameter can be implemented by a ServerHandler.
type ServerHandlerOnSetParameter interface {
	OnSetParameter(*ServerHandlerOnSetParameterCtx) (*base.Response, error)
}

// ServerHandlerOnTeardownCtx is the context of a TEARDOWN request.
type ServerHandlerOnTeardownCtx struct {
	Conn *ServerConn
	// Session *ServerSession
	Req   *base.Request
	Path  string
	Query string
}

// ServerHandlerOnTeardown can be implemented by a ServerHandler.
type ServerHandlerOnTeardown interface {
	OnTeardown(*ServerHandlerOnTeardownCtx) (*base.Response, error)
}

// ServerHandlerOnFrameCtx is the context of a frame request.
type ServerHandlerOnFrameCtx struct {
	Conn *ServerConn
	// Session *ServerSession
	TrackID    int
	StreamType StreamType
	Payload    []byte
}

// ServerHandlerOnFrame can be implemented by a ServerHandler.
type ServerHandlerOnFrame interface {
	OnFrame(*ServerHandlerOnFrameCtx)
}
