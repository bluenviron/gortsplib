package gortsplib

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/base"
)

// ServerHandler is the interface implemented by all the server handlers.
type ServerHandler interface{}

// ServerHandlerOnConnOpenCtx is the context of a connection opening.
type ServerHandlerOnConnOpenCtx struct {
	Conn *ServerConn
}

// ServerHandlerOnConnOpen can be implemented by a ServerHandler.
type ServerHandlerOnConnOpen interface {
	OnConnOpen(*ServerHandlerOnConnOpenCtx)
}

// ServerHandlerOnConnCloseCtx is the context of a connection closure.
type ServerHandlerOnConnCloseCtx struct {
	Conn  *ServerConn
	Error error
}

// ServerHandlerOnConnClose can be implemented by a ServerHandler.
type ServerHandlerOnConnClose interface {
	OnConnClose(*ServerHandlerOnConnCloseCtx)
}

// ServerHandlerOnSessionOpenCtx is the context of a session opening.
type ServerHandlerOnSessionOpenCtx struct {
	Session *ServerSession
	Conn    *ServerConn
}

// ServerHandlerOnSessionOpen can be implemented by a ServerHandler.
type ServerHandlerOnSessionOpen interface {
	OnSessionOpen(*ServerHandlerOnSessionOpenCtx)
}

// ServerHandlerOnSessionCloseCtx is the context of a session closure.
type ServerHandlerOnSessionCloseCtx struct {
	Session *ServerSession
	Error   error
}

// ServerHandlerOnSessionClose can be implemented by a ServerHandler.
type ServerHandlerOnSessionClose interface {
	OnSessionClose(*ServerHandlerOnSessionCloseCtx)
}

// ServerHandlerOnRequest can be implemented by a ServerHandler.
type ServerHandlerOnRequest interface {
	OnRequest(*ServerConn, *base.Request)
}

// ServerHandlerOnResponse can be implemented by a ServerHandler.
type ServerHandlerOnResponse interface {
	OnResponse(*ServerConn, *base.Response)
}

// ServerHandlerOnDescribeCtx is the context of a DESCRIBE request.
type ServerHandlerOnDescribeCtx struct {
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnDescribe can be implemented by a ServerHandler.
type ServerHandlerOnDescribe interface {
	OnDescribe(*ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error)
}

// ServerHandlerOnAnnounceCtx is the context of an ANNOUNCE request.
type ServerHandlerOnAnnounceCtx struct {
	Server  *Server
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
	Tracks  Tracks
}

// ServerHandlerOnAnnounce can be implemented by a ServerHandler.
type ServerHandlerOnAnnounce interface {
	OnAnnounce(*ServerHandlerOnAnnounceCtx) (*base.Response, error)
}

// ServerHandlerOnSetupCtx is the context of a OPTIONS request.
type ServerHandlerOnSetupCtx struct {
	Server    *Server
	Session   *ServerSession
	Conn      *ServerConn
	Request   *base.Request
	Path      string
	Query     string
	TrackID   int
	Transport Transport
}

// ServerHandlerOnSetup can be implemented by a ServerHandler.
type ServerHandlerOnSetup interface {
	// must return a Response and a stream.
	// the stream is needed to
	// - add the session the the stream's readers
	// - send the stream SSRC to the session
	OnSetup(*ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error)
}

// ServerHandlerOnPlayCtx is the context of a PLAY request.
type ServerHandlerOnPlayCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnPlay can be implemented by a ServerHandler.
type ServerHandlerOnPlay interface {
	OnPlay(*ServerHandlerOnPlayCtx) (*base.Response, error)
}

// ServerHandlerOnRecordCtx is the context of a RECORD request.
type ServerHandlerOnRecordCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnRecord can be implemented by a ServerHandler.
type ServerHandlerOnRecord interface {
	OnRecord(*ServerHandlerOnRecordCtx) (*base.Response, error)
}

// ServerHandlerOnPauseCtx is the context of a PAUSE request.
type ServerHandlerOnPauseCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnPause can be implemented by a ServerHandler.
type ServerHandlerOnPause interface {
	OnPause(*ServerHandlerOnPauseCtx) (*base.Response, error)
}

// ServerHandlerOnGetParameterCtx is the context of a GET_PARAMETER request.
type ServerHandlerOnGetParameterCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnGetParameter can be implemented by a ServerHandler.
type ServerHandlerOnGetParameter interface {
	OnGetParameter(*ServerHandlerOnGetParameterCtx) (*base.Response, error)
}

// ServerHandlerOnSetParameterCtx is the context of a SET_PARAMETER request.
type ServerHandlerOnSetParameterCtx struct {
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnSetParameter can be implemented by a ServerHandler.
type ServerHandlerOnSetParameter interface {
	OnSetParameter(*ServerHandlerOnSetParameterCtx) (*base.Response, error)
}

// ServerHandlerOnPacketRTPCtx is the context of a RTP packet.
type ServerHandlerOnPacketRTPCtx struct {
	Session      *ServerSession
	TrackID      int
	Packet       *rtp.Packet
	PTSEqualsDTS bool
	H264NALUs    [][]byte
	H264PTS      time.Duration
}

// ServerHandlerOnPacketRTP can be implemented by a ServerHandler.
type ServerHandlerOnPacketRTP interface {
	OnPacketRTP(*ServerHandlerOnPacketRTPCtx)
}

// ServerHandlerOnPacketRTCPCtx is the context of a RTCP packet.
type ServerHandlerOnPacketRTCPCtx struct {
	Session *ServerSession
	TrackID int
	Packet  rtcp.Packet
}

// ServerHandlerOnPacketRTCP can be implemented by a ServerHandler.
type ServerHandlerOnPacketRTCP interface {
	OnPacketRTCP(*ServerHandlerOnPacketRTCPCtx)
}
