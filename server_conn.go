package gortsplib

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"errors"
	"net"
	gourl "net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/auth"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/bytecounter"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v5/pkg/mikey"
	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
)

func getSessionID(header base.Header) string {
	if h, ok := header["Session"]; ok && len(h) == 1 {
		return h[0]
	}
	return ""
}

func checkMulticastEnabled(multicastIPRange string, query string) bool {
	// VLC uses multicast if the SDP contains a multicast address.
	// therefore, we introduce a special query (vlcmulticast) that allows
	// to return a SDP that contains a multicast address.
	if multicastIPRange != "" {
		if q, err2 := gourl.ParseQuery(query); err2 == nil {
			if _, ok := q["vlcmulticast"]; ok {
				return true
			}
		}
	}
	return false
}

func checkBackChannelsEnabled(header base.Header) bool {
	if vals, ok := header["Require"]; ok {
		for _, val := range vals {
			if val == "www.onvif.org/ver20/backchannel" {
				return true
			}
		}
	}
	return false
}

func mikeyGenerate(ctx *wrappedSRTPContext) (*mikey.Message, error) {
	csbID, err := randUint32()
	if err != nil {
		return nil, err
	}

	msg := &mikey.Message{
		Header: mikey.Header{
			Version: 1,
			CSBID:   csbID,
		},
	}

	msg.Header.CSIDMapInfo = make([]mikey.SRTPIDEntry, len(ctx.ssrcs))

	n := 0
	for _, ssrc := range ctx.ssrcs {
		msg.Header.CSIDMapInfo[n] = mikey.SRTPIDEntry{
			PolicyNo: 0,
			SSRC:     ssrc,
			ROC:      ctx.roc(ssrc),
		}
		n++
	}

	randData := make([]byte, 16)
	_, err = rand.Read(randData)
	if err != nil {
		return nil, err
	}

	msg.Payloads = []mikey.Payload{
		&mikey.PayloadT{
			TSType:  0,
			TSValue: ntp.Encode(time.Now()),
		},
		&mikey.PayloadRAND{
			Data: randData,
		},
		&mikey.PayloadSP{
			PolicyParams: []mikey.PayloadSPPolicyParam{
				{
					Type:  mikey.PayloadSPPolicyParamTypeEncrAlg,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSessionEncrKeyLen,
					Value: []byte{0x10},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeAuthAlg,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSessionAuthKeyLen,
					Value: []byte{0x0a},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSRTPEncrOffOn,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSRTCPEncrOffOn,
					Value: []byte{1},
				},
				{
					Type:  mikey.PayloadSPPolicyParamTypeSRTPAuthOffOn,
					Value: []byte{1},
				},
			},
		},
		&mikey.PayloadKEMAC{
			SubPayloads: []*mikey.SubPayloadKeyData{
				{
					Type:    mikey.SubPayloadKeyDataKeyTypeTEK,
					KeyData: ctx.key,
				},
			},
		},
	}

	return msg, nil
}

func prepareForDescribe(
	d *description.Session,
	multicast bool,
	backChannels bool,
	secure bool,
	medias map[*description.Media]*serverStreamMedia,
) (*description.Session, error) {
	out := &description.Session{
		Title:     d.Title,
		Multicast: multicast,
		FECGroups: d.FECGroups,
	}

	for i, medi := range d.Medias {
		if !medi.IsBackChannel || backChannels {
			var keyMgmtMikey *mikey.Message
			if secure {
				sm := medias[medi]

				var err error
				keyMgmtMikey, err = mikeyGenerate(sm.srtpOutCtx)
				if err != nil {
					return nil, err
				}
			}

			var profile headers.TransportProfile
			if secure {
				profile = headers.TransportProfileSAVP
			} else {
				profile = headers.TransportProfileAVP
			}

			out.Medias = append(out.Medias, &description.Media{
				Type:          medi.Type,
				ID:            medi.ID,
				IsBackChannel: medi.IsBackChannel,
				// we have to use trackID=number in order to support clients
				// like the Grandstream GXV3500.
				Control:      "trackID=" + strconv.FormatInt(int64(i), 10),
				Profile:      profile,
				KeyMgmtMikey: keyMgmtMikey,
				Formats:      medi.Formats,
			})
		}
	}

	return out, nil
}

func credentialsProvided(req *base.Request) bool {
	var auth headers.Authorization
	err := auth.Unmarshal(req.Header["Authorization"])
	return err == nil && auth.Username != ""
}

type readReq struct {
	req *base.Request
	res chan error
}

// ServerConn is a server-side RTSP connection.
type ServerConn struct {
	s      *Server
	nconn  net.Conn
	tunnel Tunnel

	ctx              context.Context
	ctxCancel        func()
	propsMutex       sync.RWMutex
	userData         interface{}
	remoteAddr       *net.TCPAddr
	bc               *bytecounter.ByteCounter
	conn             *conn.Conn
	session          *ServerSession
	authNonce        string
	httpReadBuf      *bufio.Reader
	httpReadTunnelID string

	// in
	chRequest       chan readReq
	chReadError     chan error
	chRemoveSession chan *ServerSession

	// out
	done chan struct{}
}

func (sc *ServerConn) initialize() {
	ctx, ctxCancel := context.WithCancel(sc.s.ctx)

	if sc.s.TLSConfig != nil && sc.tunnel == TunnelNone {
		sc.nconn = tls.Server(sc.nconn, sc.s.TLSConfig)
	}

	sc.bc = bytecounter.New(sc.nconn, nil, nil)
	sc.ctx = ctx
	sc.ctxCancel = ctxCancel
	sc.remoteAddr = sc.nconn.RemoteAddr().(*net.TCPAddr)
	sc.chRequest = make(chan readReq)
	sc.chReadError = make(chan error)
	sc.chRemoveSession = make(chan *ServerSession)
	sc.done = make(chan struct{})

	sc.s.wg.Add(1)
	go sc.run()
}

// Close closes the ServerConn.
func (sc *ServerConn) Close() {
	sc.ctxCancel()
}

// NetConn returns the underlying net.Conn.
func (sc *ServerConn) NetConn() net.Conn {
	return sc.nconn
}

// SetUserData sets some user data associated with the connection.
func (sc *ServerConn) SetUserData(v interface{}) {
	sc.userData = v
}

// UserData returns some user data associated with the connection.
func (sc *ServerConn) UserData() interface{} {
	return sc.userData
}

// Session returns the associated session.
func (sc *ServerConn) Session() *ServerSession {
	sc.propsMutex.RLock()
	defer sc.propsMutex.RUnlock()

	return sc.session
}

// Transport returns transport details.
func (sc *ServerConn) Transport() *ConnTransport {
	sc.propsMutex.RLock()
	defer sc.propsMutex.RUnlock()

	return &ConnTransport{
		Tunnel: sc.tunnel,
	}
}

// Stats returns connection statistics.
func (sc *ServerConn) Stats() *ConnStats {
	return &ConnStats{
		BytesReceived: sc.bc.BytesReceived(),
		BytesSent:     sc.bc.BytesSent(),
	}
}

// VerifyCredentials verifies credentials provided by the user.
func (sc *ServerConn) VerifyCredentials(
	req *base.Request,
	expectedUser string,
	expectedPass string,
) bool {
	// we do not support using an empty string as user
	// since it interferes with credentialsProvided()
	if expectedUser == "" {
		return false
	}

	if sc.authNonce == "" {
		n, err := auth.GenerateNonce()
		if err != nil {
			return false
		}
		sc.authNonce = n
	}

	err := auth.Verify(
		req,
		expectedUser,
		expectedPass,
		sc.s.AuthMethods,
		serverAuthRealm,
		sc.authNonce)

	return (err == nil)
}

func (sc *ServerConn) handleAuthError(req *base.Request, res *base.Response) error {
	// if credentials have not been provided, clear error and send the WWW-Authenticate header.
	if !credentialsProvided(req) {
		res.Header["WWW-Authenticate"] = auth.GenerateWWWAuthenticate(sc.s.AuthMethods, serverAuthRealm, sc.authNonce)
		return nil
	}

	// if credentials have been provided (and are wrong), close the connection.
	return liberrors.ErrServerAuth{}
}

func (sc *ServerConn) ip() net.IP {
	return sc.remoteAddr.IP
}

func (sc *ServerConn) zone() string {
	return sc.remoteAddr.Zone
}

func (sc *ServerConn) run() {
	defer sc.s.wg.Done()
	defer close(sc.done)

	if h, ok := sc.s.Handler.(ServerHandlerOnConnOpen); ok {
		h.OnConnOpen(&ServerHandlerOnConnOpenCtx{
			Conn: sc,
		})
	}

	reader := &serverConnReader{
		sc: sc,
	}
	reader.initialize()

	err := sc.runInner()

	sc.ctxCancel()

	if !errors.Is(err, errHTTPUpgraded) {
		sc.nconn.Close()
	}

	reader.wait()

	if sc.session != nil {
		sc.session.removeConn(sc)
	}

	sc.s.closeConn(sc)

	if h, ok := sc.s.Handler.(ServerHandlerOnConnClose); ok {
		h.OnConnClose(&ServerHandlerOnConnCloseCtx{
			Conn:  sc,
			Error: err,
		})
	}
}

func (sc *ServerConn) runInner() error {
	for {
		select {
		case req := <-sc.chRequest:
			req.res <- sc.handleRequestOuter(req.req)

		case err := <-sc.chReadError:
			return err

		case ss := <-sc.chRemoveSession:
			if sc.session == ss {
				sc.session = nil
			}

		case <-sc.ctx.Done():
			return liberrors.ErrServerTerminated{}
		}
	}
}

func (sc *ServerConn) handleRequestInner(req *base.Request) (*base.Response, error) {
	if cseq, ok := req.Header["CSeq"]; !ok || len(cseq) != 1 {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, liberrors.ErrServerCSeqMissing{}
	}

	if req.Method != base.Options && req.URL == nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, liberrors.ErrServerInvalidPath{}
	}

	sxID := getSessionID(req.Header)

	var path string
	var query string

	switch req.Method {
	case base.Describe, base.GetParameter, base.SetParameter:
		path, query = getPathAndQuery(req.URL, false)
	}

	switch req.Method {
	case base.Options:
		if sxID != "" {
			return sc.handleRequestInSession(sxID, req, false)
		}

		var methods []string
		if _, ok := sc.s.Handler.(ServerHandlerOnDescribe); ok {
			methods = append(methods, string(base.Describe))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnAnnounce); ok {
			methods = append(methods, string(base.Announce))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnSetup); ok {
			methods = append(methods, string(base.Setup))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnPlay); ok {
			methods = append(methods, string(base.Play))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnRecord); ok {
			methods = append(methods, string(base.Record))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnPause); ok {
			methods = append(methods, string(base.Pause))
		}
		methods = append(methods, string(base.GetParameter))
		if _, ok := sc.s.Handler.(ServerHandlerOnSetParameter); ok {
			methods = append(methods, string(base.SetParameter))
		}
		methods = append(methods, string(base.Teardown))

		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join(methods, ", ")},
			},
		}, nil

	case base.Describe:
		if h, ok := sc.s.Handler.(ServerHandlerOnDescribe); ok {
			res, stream, err := h.OnDescribe(&ServerHandlerOnDescribeCtx{
				Conn:    sc,
				Request: req,
				Path:    path,
				Query:   query,
			})

			if res.StatusCode == base.StatusOK {
				if stream == nil && len(res.Body) == 0 {
					panic("stream should be not nil or response body should be nonempty when StatusCode is StatusOK")
				}

				if res.Header == nil {
					res.Header = make(base.Header)
				}

				res.Header["Content-Base"] = base.HeaderValue{req.URL.String() + "/"}
				res.Header["Content-Type"] = base.HeaderValue{"application/sdp"}

				if stream == nil {
					return res, err
				}

				var desc *description.Session
				desc, err = prepareForDescribe(
					stream.Desc,
					checkMulticastEnabled(sc.s.MulticastIPRange, query),
					checkBackChannelsEnabled(req.Header),
					sc.s.TLSConfig != nil,
					stream.medias,
				)
				if err != nil {
					return &base.Response{
						StatusCode: base.StatusInternalServerError,
					}, err
				}

				var byts []byte
				byts, err = desc.Marshal()
				if err != nil {
					return &base.Response{
						StatusCode: base.StatusInternalServerError,
					}, err
				}

				res.Body = byts
			}

			return res, err
		}

	case base.Announce:
		if _, ok := sc.s.Handler.(ServerHandlerOnAnnounce); ok {
			return sc.handleRequestInSession(sxID, req, true)
		}

	case base.Setup:
		if _, ok := sc.s.Handler.(ServerHandlerOnSetup); ok {
			return sc.handleRequestInSession(sxID, req, true)
		}

	case base.Play:
		if sxID != "" {
			if _, ok := sc.s.Handler.(ServerHandlerOnPlay); ok {
				return sc.handleRequestInSession(sxID, req, false)
			}
		}

	case base.Record:
		if sxID != "" {
			if _, ok := sc.s.Handler.(ServerHandlerOnRecord); ok {
				return sc.handleRequestInSession(sxID, req, false)
			}
		}

	case base.Pause:
		if sxID != "" {
			if _, ok := sc.s.Handler.(ServerHandlerOnPause); ok {
				return sc.handleRequestInSession(sxID, req, false)
			}
		}

	case base.Teardown:
		if sxID != "" {
			return sc.handleRequestInSession(sxID, req, false)
		}

	case base.GetParameter:
		if sxID != "" {
			return sc.handleRequestInSession(sxID, req, false)
		}

		if h, ok := sc.s.Handler.(ServerHandlerOnGetParameter); ok {
			return h.OnGetParameter(&ServerHandlerOnGetParameterCtx{
				Conn:    sc,
				Request: req,
				Path:    path,
				Query:   query,
			})
		}

	case base.SetParameter:
		if sxID != "" {
			return sc.handleRequestInSession(sxID, req, false)
		}

		if h, ok := sc.s.Handler.(ServerHandlerOnSetParameter); ok {
			return h.OnSetParameter(&ServerHandlerOnSetParameterCtx{
				Conn:    sc,
				Request: req,
				Path:    path,
				Query:   query,
			})
		}
	}

	return &base.Response{
		StatusCode: base.StatusNotImplemented,
	}, nil
}

func (sc *ServerConn) handleRequestOuter(req *base.Request) error {
	if h, ok := sc.s.Handler.(ServerHandlerOnRequest); ok {
		h.OnRequest(sc, req)
	}

	res, err := sc.handleRequestInner(req)

	if res.Header == nil {
		res.Header = make(base.Header)
	}

	// handle auth errors
	var eerr1 liberrors.ErrServerAuth
	if errors.As(err, &eerr1) {
		err = sc.handleAuthError(req, res)
	}

	// add cseq
	var eerr2 liberrors.ErrServerCSeqMissing
	if !errors.As(err, &eerr2) {
		res.Header["CSeq"] = req.Header["CSeq"]
	}

	// add server
	res.Header["Server"] = base.HeaderValue{serverHeader}

	if h, ok := sc.s.Handler.(ServerHandlerOnResponse); ok {
		h.OnResponse(sc, res)
	}

	sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.WriteTimeout))
	err2 := sc.conn.WriteResponse(res)
	if err == nil && err2 != nil {
		err = err2
	}

	return err
}

func (sc *ServerConn) handleRequestInSession(
	sxID string,
	req *base.Request,
	create bool,
) (*base.Response, error) {
	// handle directly in Session
	if sc.session != nil {
		// session ID is optional in SETUP and ANNOUNCE requests, since
		// client may not have received the session ID yet due to multiple reasons:
		// * requests can be retries after code 301
		// * SETUP requests comes after ANNOUNCE response, that don't contain the session ID
		if sxID != "" {
			// the connection can't communicate with two sessions at once.
			if sxID != sc.session.secretID {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerLinkedToOtherSession{}
			}
		}

		cres := make(chan sessionRequestRes)
		sreq := sessionRequestReq{
			sc:     sc,
			req:    req,
			id:     sxID,
			create: create,
			res:    cres,
		}

		res, session, err := sc.session.handleRequest(sreq)
		sc.session = session
		return res, err
	}

	// otherwise, pass through Server
	cres := make(chan sessionRequestRes)
	sreq := sessionRequestReq{
		sc:     sc,
		req:    req,
		id:     sxID,
		create: create,
		res:    cres,
	}

	res, session, err := sc.s.handleRequest(sreq)

	sc.propsMutex.Lock()
	sc.session = session
	sc.propsMutex.Unlock()

	return res, err
}

func (sc *ServerConn) removeSession(ss *ServerSession) {
	select {
	case sc.chRemoveSession <- ss:
	case <-sc.ctx.Done():
	}
}
