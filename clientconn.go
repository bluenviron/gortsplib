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
	"sort"
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
	clientConnReadBufferSize     = 4096
	clientConnWriteBufferSize    = 4096
	clientConnCheckStreamPeriod  = 1 * time.Second
	clientConnUDPKeepalivePeriod = 30 * time.Second
)

type clientConnState int

const (
	clientConnStateInitial clientConnState = iota
	clientConnStatePrePlay
	clientConnStatePlay
	clientConnStatePreRecord
	clientConnStateRecord
)

type clientConnTrack struct {
	track           *Track
	udpRTPListener  *clientConnUDPListener
	udpRTCPListener *clientConnUDPListener
	rtcpReceiver    *rtcpreceiver.RTCPReceiver
	rtcpSender      *rtcpsender.RTCPSender
}

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
	c                 *Client
	nconn             net.Conn
	isTLS             bool
	br                *bufio.Reader
	bw                *bufio.Writer
	session           string
	cseq              int
	sender            *auth.Sender
	state             clientConnState
	streamBaseURL     *base.URL
	streamProtocol    *StreamProtocol
	tracks            map[int]clientConnTrack
	useGetParameter   bool
	writeMutex        sync.Mutex
	writeFrameAllowed bool
	writeError        error
	backgroundRunning bool
	readCB            func(int, StreamType, []byte)

	// TCP stream protocol
	tcpFrameBuffer *multibuffer.MultiBuffer

	// read
	rtpInfo *headers.RTPInfo

	// in
	backgroundTerminate chan struct{}

	// out
	backgroundDone chan struct{}
}

func newClientConn(c *Client, scheme string, host string) (*ClientConn, error) {
	// connection
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 10 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// reading / writing
	if c.InitialUDPReadTimeout == 0 {
		c.InitialUDPReadTimeout = 3 * time.Second
	}

	if c.ReadBufferCount == 0 {
		c.ReadBufferCount = 1
	}
	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = 2048
	}

	// system functions
	if c.DialTimeout == nil {
		c.DialTimeout = net.DialTimeout
	}
	if c.ListenPacket == nil {
		c.ListenPacket = net.ListenPacket
	}

	// private
	if c.senderReportPeriod == 0 {
		c.senderReportPeriod = 10 * time.Second
	}
	if c.receiverReportPeriod == 0 {
		c.receiverReportPeriod = 10 * time.Second
	}

	cc := &ClientConn{
		c:          c,
		tracks:     make(map[int]clientConnTrack),
		writeError: fmt.Errorf("not running"),
	}

	err := cc.connOpen(scheme, host)
	if err != nil {
		return nil, err
	}

	return cc, nil
}

// Close closes all the ClientConn resources.
func (cc *ClientConn) Close() error {
	if cc.backgroundRunning {
		close(cc.backgroundTerminate)
		<-cc.backgroundDone
	}

	if cc.state == clientConnStatePlay || cc.state == clientConnStateRecord {
		cc.Do(&base.Request{
			Method:       base.Teardown,
			URL:          cc.streamBaseURL,
			SkipResponse: true,
		})
	}

	for _, track := range cc.tracks {
		if track.udpRTPListener != nil {
			track.udpRTPListener.close()
			track.udpRTCPListener.close()
		}
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
	cc.streamBaseURL = nil
	cc.streamProtocol = nil
	cc.tracks = make(map[int]clientConnTrack)
	cc.useGetParameter = false
	cc.backgroundRunning = false

	// read
	cc.rtpInfo = nil
	cc.tcpFrameBuffer = nil
	cc.readCB = nil
}

func (cc *ClientConn) connOpen(scheme string, host string) error {
	if scheme != "rtsp" && scheme != "rtsps" {
		return fmt.Errorf("unsupported scheme '%s'", scheme)
	}

	v := StreamProtocolUDP
	if scheme == "rtsps" && cc.c.StreamProtocol == &v {
		return fmt.Errorf("RTSPS can't be used with UDP")
	}

	if !strings.Contains(host, ":") {
		host += ":554"
	}

	nconn, err := cc.c.DialTimeout("tcp", host, cc.c.ReadTimeout)
	if err != nil {
		return err
	}

	conn := func() net.Conn {
		if scheme == "rtsps" {
			return tls.Client(nconn, cc.c.TLSConfig)
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
	var ret Tracks

	for _, track := range cc.tracks {
		ret = append(ret, track.track)
	}

	// sort by ID to generate correct SDPs
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ID < ret[j].ID
	})

	return ret
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

	if cc.c.OnRequest != nil {
		cc.c.OnRequest(req)
	}

	cc.nconn.SetWriteDeadline(time.Now().Add(cc.c.WriteTimeout))
	err := req.Write(cc.bw)
	if err != nil {
		return nil, err
	}

	if req.SkipResponse {
		return nil, nil
	}

	var res base.Response
	cc.nconn.SetReadDeadline(time.Now().Add(cc.c.ReadTimeout))

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

	if cc.c.OnResponse != nil {
		cc.c.OnResponse(&res)
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

	cc.useGetParameter = func() bool {
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
		if !cc.c.RedirectDisable &&
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

	baseURL, err := func() (*base.URL, error) {
		// prefer Content-Base (optional)
		if cb, ok := res.Header["Content-Base"]; ok {
			if len(cb) != 1 {
				return nil, fmt.Errorf("invalid Content-Base: '%v'", cb)
			}

			ret, err := base.ParseURL(cb[0])
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Base: '%v'", cb)
			}

			return ret, nil
		}

		// if not provided, use DESCRIBE URL
		return u, nil
	}()
	if err != nil {
		return nil, nil, err
	}

	tracks, err := ReadTracks(res.Body, baseURL)
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

	if cc.streamBaseURL != nil && *track.BaseURL != *cc.streamBaseURL {
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
		if cc.c.StreamProtocol != nil {
			return *cc.c.StreamProtocol
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
			cc.c.StreamProtocol == nil {

			v := StreamProtocolTCP
			cc.streamProtocol = &v

			return cc.Setup(mode, track, 0, 0)
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

		if !cc.c.AnyPortEnable {
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
	cct := clientConnTrack{
		track: track,
	}

	if mode == headers.TransportModePlay {
		cct.rtcpReceiver = rtcpreceiver.New(nil, clockRate)
	} else {
		cct.rtcpSender = rtcpsender.New(clockRate)
	}

	cc.streamBaseURL = track.BaseURL
	cc.streamProtocol = &proto

	if proto == StreamProtocolUDP {
		rtpListener.remoteIP = cc.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = cc.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtpListener.remotePort = thRes.ServerPorts[0]
		}
		rtpListener.trackID = track.ID
		rtpListener.streamType = StreamTypeRTP
		cct.udpRTPListener = rtpListener

		rtcpListener.remoteIP = cc.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = cc.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtcpListener.remotePort = thRes.ServerPorts[1]
		}
		rtcpListener.trackID = track.ID
		rtcpListener.streamType = StreamTypeRTCP
		cct.udpRTCPListener = rtcpListener
	}

	cc.tracks[track.ID] = cct

	if mode == headers.TransportModePlay {
		cc.state = clientConnStatePrePlay
	} else {
		cc.state = clientConnStatePreRecord
	}

	if *cc.streamProtocol == StreamProtocolTCP &&
		cc.tcpFrameBuffer == nil {
		cc.tcpFrameBuffer = multibuffer.New(uint64(cc.c.ReadBufferCount), uint64(cc.c.ReadBufferSize))
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
	cc.backgroundRunning = false

	res, err := cc.Do(&base.Request{
		Method: base.Pause,
		URL:    cc.streamBaseURL,
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

// WriteFrame writes a frame.
func (cc *ClientConn) WriteFrame(trackID int, streamType StreamType, payload []byte) error {
	now := time.Now()

	cc.writeMutex.Lock()
	defer cc.writeMutex.Unlock()

	if !cc.writeFrameAllowed {
		return cc.writeError
	}

	if cc.tracks[trackID].rtcpSender != nil {
		cc.tracks[trackID].rtcpSender.ProcessFrame(now, streamType, payload)
	}

	if *cc.streamProtocol == StreamProtocolUDP {
		if streamType == StreamTypeRTP {
			return cc.tracks[trackID].udpRTPListener.write(payload)
		}
		return cc.tracks[trackID].udpRTCPListener.write(payload)
	}

	cc.nconn.SetWriteDeadline(now.Add(cc.c.WriteTimeout))
	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Payload:    payload,
	}
	return frame.Write(cc.bw)
}

// ReadFrames starts reading frames.
// it returns a channel that is written when the reading stops.
func (cc *ClientConn) ReadFrames(onFrame func(int, StreamType, []byte)) chan error {
	// channel is buffered, since listening to it is not mandatory
	done := make(chan error, 1)

	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePlay:   {},
		clientConnStateRecord: {},
	})
	if err != nil {
		done <- err
		return done
	}

	// close previous ReadFrames()
	if cc.backgroundRunning {
		close(cc.backgroundTerminate)
		<-cc.backgroundDone
	}

	cc.backgroundRunning = true
	cc.backgroundTerminate = make(chan struct{})
	cc.backgroundDone = make(chan struct{})
	cc.readCB = onFrame
	cc.writeFrameAllowed = true

	go func() {
		done <- func() error {
			safeState := cc.state
			err := func() error {
				if *cc.streamProtocol == StreamProtocolUDP {
					if cc.state == clientConnStatePlay {
						return cc.backgroundPlayUDP()
					}
					return cc.backgroundRecordUDP()
				}

				if cc.state == clientConnStatePlay {
					return cc.backgroundPlayTCP()
				}
				return cc.backgroundRecordTCP()
			}()

			cc.writeError = err

			func() {
				cc.writeMutex.Lock()
				defer cc.writeMutex.Unlock()
				cc.writeFrameAllowed = false
			}()

			close(cc.backgroundDone)

			// automatically change protocol in case of timeout
			if *cc.streamProtocol == StreamProtocolUDP &&
				safeState == clientConnStatePlay {
				if _, ok := err.(liberrors.ErrClientNoUDPPacketsRecently); ok {
					if cc.c.StreamProtocol == nil {
						prevBaseURL := cc.streamBaseURL
						oldUseGetParameter := cc.useGetParameter
						prevTracks := cc.tracks
						cc.reset()
						v := StreamProtocolTCP
						cc.streamProtocol = &v
						cc.useGetParameter = oldUseGetParameter

						err := cc.connOpen(prevBaseURL.Scheme, prevBaseURL.Host)
						if err != nil {
							return err
						}

						for _, track := range prevTracks {
							_, err := cc.Setup(headers.TransportModePlay, track.track, 0, 0)
							if err != nil {
								cc.Close()
								return err
							}
						}

						_, err = cc.Play()
						if err != nil {
							cc.Close()
							return err
						}

						return <-cc.ReadFrames(onFrame)
					}
				}
			}

			return err
		}()
	}()

	return done
}
