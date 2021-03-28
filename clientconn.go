/*
Package gortsplib is a RTSP 1.0 library for the Go programming language,
written for rtsp-simple-server.

Examples are available at https://github.com/aler9/gortsplib/tree/master/examples

*/
package gortsplib

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aler9/gortsplib/pkg/auth"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
)

const (
	clientConnReadBufferSize       = 4096
	clientConnWriteBufferSize      = 4096
	clientConnReceiverReportPeriod = 10 * time.Second
	clientConnSenderReportPeriod   = 10 * time.Second
	clientConnUDPCheckStreamPeriod = 1 * time.Second
	clientConnUDPKeepalivePeriod   = 30 * time.Second
	clientConnTCPSetDeadlinePeriod = 1 * time.Second
)

type clientConnState int

const (
	clientConnStateInitial clientConnState = iota
	clientConnStatePrePlay
	clientConnStatePlay
	clientConnStatePreRecord
	clientConnStateRecord
)

func (s clientConnState) String() string {
	switch s {
	case clientConnStateInitial:
		return "initial"
	case clientConnStatePrePlay:
		return "prePlay"
	case clientConnStatePlay:
		return "play"
	case clientConnStatePreRecord:
		return "preRecord"
	case clientConnStateRecord:
		return "record"
	}
	return "unknown"
}

// ClientConn is a client-side RTSP connection.
type ClientConn struct {
	conf                  ClientConf
	nconn                 net.Conn
	isTLS                 bool
	br                    *bufio.Reader
	bw                    *bufio.Writer
	session               string
	cseq                  int
	sender                *auth.Sender
	state                 clientConnState
	streamURL             *base.URL
	streamProtocol        *StreamProtocol
	tracks                Tracks
	udpRTPListeners       map[int]*clientConnUDPListener
	udpRTCPListeners      map[int]*clientConnUDPListener
	getParameterSupported bool

	// read only
	rtpInfo        *headers.RTPInfo
	rtcpReceivers  map[int]*rtcpreceiver.RTCPReceiver
	tcpFrameBuffer *multibuffer.MultiBuffer
	readCB         func(int, StreamType, []byte)

	// publish only
	rtcpSenders       map[int]*rtcpsender.RTCPSender
	publishError      error
	publishWriteMutex sync.RWMutex
	publishOpen       bool

	// in
	backgroundTerminate chan struct{}

	// out
	backgroundDone chan struct{}
}

func newClientConn(conf ClientConf, scheme string, host string) (*ClientConn, error) {
	if conf.TLSConfig == nil {
		conf.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if conf.ReadTimeout == 0 {
		conf.ReadTimeout = 10 * time.Second
	}
	if conf.InitialUDPReadTimeout == 0 {
		conf.InitialUDPReadTimeout = 3 * time.Second
	}
	if conf.WriteTimeout == 0 {
		conf.WriteTimeout = 10 * time.Second
	}
	if conf.ReadBufferCount == 0 {
		conf.ReadBufferCount = 1
	}

	if conf.ReadBufferSize == 0 {
		conf.ReadBufferSize = 2048
	}
	if conf.DialTimeout == nil {
		conf.DialTimeout = net.DialTimeout
	}
	if conf.ListenPacket == nil {
		conf.ListenPacket = net.ListenPacket
	}

	cc := &ClientConn{
		conf:             conf,
		udpRTPListeners:  make(map[int]*clientConnUDPListener),
		udpRTCPListeners: make(map[int]*clientConnUDPListener),
		publishError:     fmt.Errorf("not running"),
	}

	err := cc.connOpen(scheme, host)
	if err != nil {
		return nil, err
	}

	return cc, nil
}

// Close closes all the ClientConn resources.
func (cc *ClientConn) Close() error {
	if cc.state == clientConnStatePlay || cc.state == clientConnStateRecord {
		close(cc.backgroundTerminate)
		<-cc.backgroundDone

		cc.Do(&base.Request{
			Method:       base.Teardown,
			URL:          cc.streamURL,
			SkipResponse: true,
		})
	}

	for _, l := range cc.udpRTPListeners {
		l.close()
	}

	for _, l := range cc.udpRTCPListeners {
		l.close()
	}

	if cc.nconn != nil {
		cc.nconn.Close()
	}

	return nil
}

func (cc *ClientConn) reset() {
	cc.Close()

	cc.state = clientConnStateInitial
	cc.nconn = nil
	cc.streamURL = nil
	cc.streamProtocol = nil
	cc.tracks = nil
	cc.udpRTPListeners = make(map[int]*clientConnUDPListener)
	cc.udpRTCPListeners = make(map[int]*clientConnUDPListener)
	cc.getParameterSupported = false

	// read only
	cc.rtpInfo = nil
	cc.rtcpReceivers = nil
	cc.tcpFrameBuffer = nil
	cc.readCB = nil
}

func (cc *ClientConn) connOpen(scheme string, host string) error {
	if scheme != "rtsp" && scheme != "rtsps" {
		return fmt.Errorf("unsupported scheme '%s'", scheme)
	}

	v := StreamProtocolUDP
	if scheme == "rtsps" && cc.conf.StreamProtocol == &v {
		return fmt.Errorf("RTSPS can't be used with UDP")
	}

	if !strings.Contains(host, ":") {
		host += ":554"
	}

	nconn, err := cc.conf.DialTimeout("tcp", host, cc.conf.ReadTimeout)
	if err != nil {
		return err
	}

	conn := func() net.Conn {
		if scheme == "rtsps" {
			return tls.Client(nconn, cc.conf.TLSConfig)
		}
		return nconn
	}()

	cc.nconn = nconn
	cc.isTLS = (scheme == "rtsps")
	cc.br = bufio.NewReaderSize(conn, clientConnReadBufferSize)
	cc.bw = bufio.NewWriterSize(conn, clientConnWriteBufferSize)
	return nil
}

func (cc *ClientConn) checkState(allowed map[clientConnState]struct{}) error {
	if _, ok := allowed[cc.state]; ok {
		return nil
	}

	allowedList := make([]fmt.Stringer, len(allowed))
	i := 0
	for a := range allowed {
		allowedList[i] = a
		i++
	}
	return liberrors.ErrClientWrongState{AllowedList: allowedList, State: cc.state}
}

// NetConn returns the underlying net.Conn.
func (cc *ClientConn) NetConn() net.Conn {
	return cc.nconn
}

// Tracks returns all the tracks that the connection is reading or publishing.
func (cc *ClientConn) Tracks() Tracks {
	return cc.tracks
}

// Do writes a Request and reads a Response.
// Interleaved frames received before the response are ignored.
func (cc *ClientConn) Do(req *base.Request) (*base.Response, error) {
	if req.Header == nil {
		req.Header = make(base.Header)
	}

	// add session
	if cc.session != "" {
		req.Header["Session"] = base.HeaderValue{cc.session}
	}

	// add auth
	if cc.sender != nil {
		req.Header["Authorization"] = cc.sender.GenerateHeader(req.Method, req.URL)
	}

	// add cseq
	cc.cseq++
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(cc.cseq), 10)}

	// add user agent
	req.Header["User-Agent"] = base.HeaderValue{"gortsplib"}

	if cc.conf.OnRequest != nil {
		cc.conf.OnRequest(req)
	}

	cc.nconn.SetWriteDeadline(time.Now().Add(cc.conf.WriteTimeout))
	err := req.Write(cc.bw)
	if err != nil {
		return nil, err
	}

	if req.SkipResponse {
		return nil, nil
	}

	var res base.Response
	cc.nconn.SetReadDeadline(time.Now().Add(cc.conf.ReadTimeout))

	if cc.tcpFrameBuffer != nil {
		// read the response and ignore interleaved frames in between;
		// interleaved frames are sent in two scenarios:
		// * when the server is v4lrtspserver, before the PLAY response
		// * when the stream is already playing
		err = res.ReadIgnoreFrames(cc.br, cc.tcpFrameBuffer.Next())
		if err != nil {
			return nil, err
		}
	} else {
		err = res.Read(cc.br)
		if err != nil {
			return nil, err
		}
	}

	if cc.conf.OnResponse != nil {
		cc.conf.OnResponse(&res)
	}

	// get session from response
	if v, ok := res.Header["Session"]; ok {
		var sx headers.Session
		err := sx.Read(v)
		if err != nil {
			return nil, liberrors.ErrClientSessionHeaderInvalid{Err: err}
		}
		cc.session = sx.Session
	}

	// setup authentication
	if res.StatusCode == base.StatusUnauthorized && req.URL.User != nil && cc.sender == nil {
		pass, _ := req.URL.User.Password()
		user := req.URL.User.Username()

		sender, err := auth.NewSender(res.Header["WWW-Authenticate"], user, pass)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		cc.sender = sender

		// send request again
		return cc.Do(req)
	}

	return &res, nil
}

// Options writes an OPTIONS request and reads a response.
func (cc *ClientConn) Options(u *base.URL) (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := cc.Do(&base.Request{
		Method: base.Options,
		URL:    u,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		// since this method is not implemented by every RTSP server,
		// return only if status code is not 404
		if res.StatusCode == base.StatusNotFound {
			return res, nil
		}
		return res, liberrors.ErrClientWrongStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	cc.getParameterSupported = func() bool {
		pub, ok := res.Header["Public"]
		if !ok || len(pub) != 1 {
			return false
		}

		for _, m := range strings.Split(pub[0], ",") {
			if base.Method(m) == base.GetParameter {
				return true
			}
		}
		return false
	}()

	return res, nil
}

// Describe writes a DESCRIBE request and reads a Response.
func (cc *ClientConn) Describe(u *base.URL) (Tracks, *base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, nil, err
	}

	res, err := cc.Do(&base.Request{
		Method: base.Describe,
		URL:    u,
		Header: base.Header{
			"Accept": base.HeaderValue{"application/sdp"},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode != base.StatusOK {
		// redirect
		if !cc.conf.RedirectDisable &&
			res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {

			cc.reset()

			u, err := base.ParseURL(res.Header["Location"][0])
			if err != nil {
				return nil, nil, err
			}

			err = cc.connOpen(u.Scheme, u.Host)
			if err != nil {
				return nil, nil, err
			}

			_, err = cc.Options(u)
			if err != nil {
				return nil, nil, err
			}

			return cc.Describe(u)
		}

		return nil, res, liberrors.ErrClientWrongStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	ct, ok := res.Header["Content-Type"]
	if !ok || len(ct) != 1 {
		return nil, nil, liberrors.ErrClientContentTypeMissing{}
	}

	if ct[0] != "application/sdp" {
		return nil, nil, liberrors.ErrClientContentTypeUnsupported{CT: ct}
	}

	tracks, err := ReadTracks(res.Body, u)
	if err != nil {
		return nil, nil, err
	}

	return tracks, res, nil
}

// Setup writes a SETUP request and reads a Response.
// rtpPort and rtcpPort are used only if protocol is UDP.
// if rtpPort and rtcpPort are zero, they are chosen automatically.
func (cc *ClientConn) Setup(mode headers.TransportMode, track *Track,
	rtpPort int, rtcpPort int) (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	if (mode == headers.TransportModeRecord && cc.state != clientConnStatePreRecord) ||
		(mode == headers.TransportModePlay && cc.state != clientConnStatePrePlay &&
			cc.state != clientConnStateInitial) {
		return nil, liberrors.ErrClientCannotReadPublishAtSameTime{}
	}

	if cc.streamURL != nil && *track.BaseURL != *cc.streamURL {
		return nil, liberrors.ErrClientCannotSetupTracksDifferentURLs{}
	}

	var rtpListener *clientConnUDPListener
	var rtcpListener *clientConnUDPListener

	// always use TCP if encrypted
	if cc.isTLS {
		v := StreamProtocolTCP
		cc.streamProtocol = &v
	}

	proto := func() StreamProtocol {
		// protocol set by previous Setup() or ReadFrames()
		if cc.streamProtocol != nil {
			return *cc.streamProtocol
		}

		// protocol set by conf
		if cc.conf.StreamProtocol != nil {
			return *cc.conf.StreamProtocol
		}

		// try UDP
		return StreamProtocolUDP
	}()

	th := headers.Transport{
		Protocol: proto,
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: &mode,
	}

	if proto == base.StreamProtocolUDP {
		if (rtpPort == 0 && rtcpPort != 0) ||
			(rtpPort != 0 && rtcpPort == 0) {
			return nil, liberrors.ErrClientUDPPortsZero{}
		}

		if rtpPort != 0 && rtcpPort != (rtpPort+1) {
			return nil, liberrors.ErrClientUDPPortsNotConsecutive{}
		}

		var err error
		rtpListener, rtcpListener, err = func() (*clientConnUDPListener, *clientConnUDPListener, error) {
			if rtpPort != 0 {
				rtpListener, err := newClientConnUDPListener(cc, rtpPort)
				if err != nil {
					return nil, nil, err
				}

				rtcpListener, err := newClientConnUDPListener(cc, rtcpPort)
				if err != nil {
					rtpListener.close()
					return nil, nil, err
				}

				return rtpListener, rtcpListener, nil
			}

			// choose two consecutive ports in range 65535-10000
			// rtp must be even and rtcp odd
			for {
				rtpPort = (rand.Intn((65535-10000)/2) * 2) + 10000
				rtcpPort = rtpPort + 1

				rtpListener, err := newClientConnUDPListener(cc, rtpPort)
				if err != nil {
					continue
				}

				rtcpListener, err := newClientConnUDPListener(cc, rtcpPort)
				if err != nil {
					rtpListener.close()
					continue
				}

				return rtpListener, rtcpListener, nil
			}
		}()
		if err != nil {
			return nil, err
		}

		th.ClientPorts = &[2]int{rtpPort, rtcpPort}

	} else {
		th.InterleavedIDs = &[2]int{(track.ID * 2), (track.ID * 2) + 1}
	}

	trackURL, err := track.URL()
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, err
	}

	res, err := cc.Do(&base.Request{
		Method: base.Setup,
		URL:    trackURL,
		Header: base.Header{
			"Transport": th.Write(),
		},
	})
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}

		// switch protocol automatically
		if res.StatusCode == base.StatusUnsupportedTransport &&
			cc.streamProtocol == nil &&
			cc.conf.StreamProtocol == nil {

			v := StreamProtocolTCP
			cc.streamProtocol = &v

			return cc.Setup(headers.TransportModePlay, track, 0, 0)
		}

		return res, liberrors.ErrClientWrongStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	var thRes headers.Transport
	err = thRes.Read(res.Header["Transport"])
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, liberrors.ErrClientTransportHeaderInvalid{Err: err}
	}

	if proto == StreamProtocolUDP {
		if thRes.ServerPorts != nil {
			if (thRes.ServerPorts[0] == 0 && thRes.ServerPorts[1] != 0) ||
				(thRes.ServerPorts[0] != 0 && thRes.ServerPorts[1] == 0) {
				rtpListener.close()
				rtcpListener.close()
				return nil, liberrors.ErrClientServerPortsZero{}
			}
		}

		if !cc.conf.AnyPortEnable {
			if thRes.ServerPorts == nil || (thRes.ServerPorts[0] == 0 && thRes.ServerPorts[1] == 0) {
				rtpListener.close()
				rtcpListener.close()
				return nil, liberrors.ErrClientServerPortsNotProvided{}
			}
		}

	} else {
		if thRes.InterleavedIDs == nil {
			return nil, liberrors.ErrClientTransportHeaderNoInterleavedIDs{}
		}

		if thRes.InterleavedIDs[0] != th.InterleavedIDs[0] ||
			thRes.InterleavedIDs[1] != th.InterleavedIDs[1] {
			return nil, liberrors.ErrClientTransportHeaderWrongInterleavedIDs{
				Expected: *th.InterleavedIDs, Value: *thRes.InterleavedIDs}
		}
	}

	clockRate, _ := track.ClockRate()

	if mode == headers.TransportModePlay {
		if cc.rtcpReceivers == nil {
			cc.rtcpReceivers = make(map[int]*rtcpreceiver.RTCPReceiver)
		}
		cc.rtcpReceivers[track.ID] = rtcpreceiver.New(nil, clockRate)
	} else {
		if cc.rtcpSenders == nil {
			cc.rtcpSenders = make(map[int]*rtcpsender.RTCPSender)
		}
		cc.rtcpSenders[track.ID] = rtcpsender.New(clockRate)
	}

	cc.streamURL = track.BaseURL
	cc.streamProtocol = &proto
	cc.tracks = append(cc.tracks, track)

	if proto == StreamProtocolUDP {
		rtpListener.remoteIP = cc.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = cc.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtpListener.remotePort = thRes.ServerPorts[0]
		}
		rtpListener.trackID = track.ID
		rtpListener.streamType = StreamTypeRTP
		cc.udpRTPListeners[track.ID] = rtpListener

		rtcpListener.remoteIP = cc.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = cc.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtcpListener.remotePort = thRes.ServerPorts[1]
		}
		rtcpListener.trackID = track.ID
		rtcpListener.streamType = StreamTypeRTCP
		cc.udpRTCPListeners[track.ID] = rtcpListener
	}

	if mode == headers.TransportModePlay {
		cc.state = clientConnStatePrePlay

		if *cc.streamProtocol == StreamProtocolTCP && cc.tcpFrameBuffer == nil {
			cc.tcpFrameBuffer = multibuffer.New(uint64(cc.conf.ReadBufferCount), uint64(cc.conf.ReadBufferSize))
		}

	} else {
		cc.state = clientConnStatePreRecord
	}

	return res, nil
}

// Pause writes a PAUSE request and reads a Response.
// This can be called only after Play() or Record().
func (cc *ClientConn) Pause() (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePlay:   {},
		clientConnStateRecord: {},
	})
	if err != nil {
		return nil, err
	}

	close(cc.backgroundTerminate)
	<-cc.backgroundDone

	res, err := cc.Do(&base.Request{
		Method: base.Pause,
		URL:    cc.streamURL,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return res, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage}
	}

	switch cc.state {
	case clientConnStatePlay:
		cc.state = clientConnStatePrePlay
	case clientConnStateRecord:
		cc.state = clientConnStatePreRecord
	}

	return res, nil
}
