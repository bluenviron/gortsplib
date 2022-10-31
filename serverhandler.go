package gortsplib

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/base"
)

// ServerHandler is the interface implemented by all the server handlers.
type ServerHandler interface{}

// ServerHandlerOnConnOpenCtx is the context of OnConnOpen.
type ServerHandlerOnConnOpenCtx struct {
	Conn *ServerConn
}

// ServerHandlerOnConnOpen can be implemented by a ServerHandler.
type ServerHandlerOnConnOpen interface {
	// called when a connection is opened.
	OnConnOpen(*ServerHandlerOnConnOpenCtx)
}

// ServerHandlerOnConnCloseCtx is the context of OnConnClose.
type ServerHandlerOnConnCloseCtx struct {
	Conn  *ServerConn
	Error error
}

// ServerHandlerOnConnClose can be implemented by a ServerHandler.
type ServerHandlerOnConnClose interface {
	// called when a connection is closed.
	OnConnClose(*ServerHandlerOnConnCloseCtx)
}

// ServerHandlerOnSessionOpenCtx is the context OnSessionOpen.
type ServerHandlerOnSessionOpenCtx struct {
	Session *ServerSession
	Conn    *ServerConn
}

// ServerHandlerOnSessionOpen can be implemented by a ServerHandler.
type ServerHandlerOnSessionOpen interface {
	// called when a session is opened.
	OnSessionOpen(*ServerHandlerOnSessionOpenCtx)
}

// ServerHandlerOnSessionCloseCtx is the context of ServerHandlerOnSessionClose.
type ServerHandlerOnSessionCloseCtx struct {
	Session *ServerSession
	Error   error
}

// ServerHandlerOnSessionClose can be implemented by a ServerHandler.
type ServerHandlerOnSessionClose interface {
	// called when a session is closed.
	OnSessionClose(*ServerHandlerOnSessionCloseCtx)
}

// ServerHandlerOnRequest can be implemented by a ServerHandler.
type ServerHandlerOnRequest interface {
	// called before every request.
	OnRequest(*ServerConn, *base.Request)
}

// ServerHandlerOnResponse can be implemented by a ServerHandler.
type ServerHandlerOnResponse interface {
	// called after every response.
	OnResponse(*ServerConn, *base.Response)
}

// ServerHandlerOnDescribeCtx is the context of OnDescribe.
type ServerHandlerOnDescribeCtx struct {
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnDescribe can be implemented by a ServerHandler.
type ServerHandlerOnDescribe interface {
	// called after receiving a DESCRIBE request.
	OnDescribe(*ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error)
}

// ServerHandlerOnAnnounceCtx is the context of OnAnnounce.
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
	// called after receiving an ANNOUNCE request.
	OnAnnounce(*ServerHandlerOnAnnounceCtx) (*base.Response, error)
}

// ServerHandlerOnSetupCtx is the context of OnSetup.
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
	// called after receiving a SETUP request.
	// must return a Response and a stream.
	// the stream is needed to
	// - add the session the the stream's readers
	// - send the stream SSRC to the session
	OnSetup(*ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error)
}

// ServerHandlerOnPlayCtx is the context of OnPlay.
type ServerHandlerOnPlayCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnPlay can be implemented by a ServerHandler.
type ServerHandlerOnPlay interface {
	// called after receiving a PLAY request.
	OnPlay(*ServerHandlerOnPlayCtx) (*base.Response, error)
}

// ServerHandlerOnRecordCtx is the context of OnRecord.
type ServerHandlerOnRecordCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnRecord can be implemented by a ServerHandler.
type ServerHandlerOnRecord interface {
	// called after receiving a RECORD request.
	OnRecord(*ServerHandlerOnRecordCtx) (*base.Response, error)
}

// ServerHandlerOnPauseCtx is the context of OnPause.
type ServerHandlerOnPauseCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnPause can be implemented by a ServerHandler.
type ServerHandlerOnPause interface {
	// called after receiving a PAUSE request.
	OnPause(*ServerHandlerOnPauseCtx) (*base.Response, error)
}

// ServerHandlerOnGetParameterCtx is the context of OnGetParameter.
type ServerHandlerOnGetParameterCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnGetParameter can be implemented by a ServerHandler.
type ServerHandlerOnGetParameter interface {
	// called after receiving a GET_PARAMETER request.
	OnGetParameter(*ServerHandlerOnGetParameterCtx) (*base.Response, error)
}

// ServerHandlerOnSetParameterCtx is the context of OnSetParameter.
type ServerHandlerOnSetParameterCtx struct {
	Session *ServerSession
	Conn    *ServerConn
	Request *base.Request
	Path    string
	Query   string
}

// ServerHandlerOnSetParameter can be implemented by a ServerHandler.
type ServerHandlerOnSetParameter interface {
	// called after receiving a SET_PARAMETER request.
	OnSetParameter(*ServerHandlerOnSetParameterCtx) (*base.Response, error)
}

// ServerHandlerOnPacketRTPCtx is the context of OnPacketRTP.
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
	// called when receiving a RTP packet.
	OnPacketRTP(*ServerHandlerOnPacketRTPCtx)
}

// ServerHandlerOnPacketRTCPCtx is the context of OnPacketRTCP.
type ServerHandlerOnPacketRTCPCtx struct {
	Session *ServerSession
	TrackID int
	Packet  rtcp.Packet
}

// ServerHandlerOnPacketRTCP can be implemented by a ServerHandler.
type ServerHandlerOnPacketRTCP interface {
	// called when receiving a RTCP packet.
	OnPacketRTCP(*ServerHandlerOnPacketRTCPCtx)
}
