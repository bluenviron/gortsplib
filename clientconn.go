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
	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
)

const (
	clientConnReadBufferSize       = 4096
	clientConnWriteBufferSize      = 4096
	clientConnReceiverReportPeriod = 10 * time.Second
	clientConnSenderReportPeriod   = 10 * time.Second
	clientConnUDPCheckStreamPeriod = 5 * time.Second
	clientConnUDPKeepalivePeriod   = 30 * time.Second
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
	rtcpReceivers     map[int]*rtcpreceiver.RTCPReceiver
	udpLastFrameTimes map[int]*int64
	tcpFrameBuffer    *multibuffer.MultiBuffer
	readCB            func(int, StreamType, []byte)

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

	if scheme != "rtsp" && scheme != "rtsps" {
		return nil, fmt.Errorf("unsupported scheme '%s'", scheme)
	}

	v := StreamProtocolUDP
	if scheme == "rtsps" && conf.StreamProtocol == &v {
		return nil, fmt.Errorf("RTSPS can't be used with UDP")
	}

	if !strings.Contains(host, ":") {
		host += ":554"
	}

	nconn, err := conf.DialTimeout("tcp", host, conf.ReadTimeout)
	if err != nil {
		return nil, err
	}

	conn := func() net.Conn {
		if scheme == "rtsps" {
			return tls.Client(nconn, conf.TLSConfig)
		}
		return nconn
	}()

	return &ClientConn{
		conf:              conf,
		nconn:             nconn,
		isTLS:             (scheme == "rtsps"),
		br:                bufio.NewReaderSize(conn, clientConnReadBufferSize),
		bw:                bufio.NewWriterSize(conn, clientConnWriteBufferSize),
		udpRTPListeners:   make(map[int]*clientConnUDPListener),
		udpRTCPListeners:  make(map[int]*clientConnUDPListener),
		rtcpReceivers:     make(map[int]*rtcpreceiver.RTCPReceiver),
		udpLastFrameTimes: make(map[int]*int64),
		tcpFrameBuffer:    multibuffer.New(uint64(conf.ReadBufferCount), uint64(conf.ReadBufferSize)),
		rtcpSenders:       make(map[int]*rtcpsender.RTCPSender),
		publishError:      fmt.Errorf("not running"),
	}, nil
}

// Close closes all the ClientConn resources.
func (c *ClientConn) Close() error {
	if c.state == clientConnStatePlay || c.state == clientConnStateRecord {
		close(c.backgroundTerminate)
		<-c.backgroundDone

		c.Do(&base.Request{
			Method:       base.Teardown,
			URL:          c.streamURL,
			SkipResponse: true,
		})
	}

	for _, l := range c.udpRTPListeners {
		l.close()
	}

	for _, l := range c.udpRTCPListeners {
		l.close()
	}

	err := c.nconn.Close()
	return err
}

func (c *ClientConn) checkState(allowed map[clientConnState]struct{}) error {
	if _, ok := allowed[c.state]; ok {
		return nil
	}

	var allowedList []clientConnState
	for a := range allowed {
		allowedList = append(allowedList, a)
	}
	return fmt.Errorf("must be in state %v, while is in state %v",
		allowedList, c.state)
}

// NetConn returns the underlying net.Conn.
func (c *ClientConn) NetConn() net.Conn {
	return c.nconn
}

// Tracks returns all the tracks that the connection is reading or publishing.
func (c *ClientConn) Tracks() Tracks {
	return c.tracks
}

// Do writes a Request and reads a Response.
// Interleaved frames received before the response are ignored.
func (c *ClientConn) Do(req *base.Request) (*base.Response, error) {
	if req.Header == nil {
		req.Header = make(base.Header)
	}

	// add session
	if c.session != "" {
		req.Header["Session"] = base.HeaderValue{c.session}
	}

	// add auth
	if c.sender != nil {
		req.Header["Authorization"] = c.sender.GenerateHeader(req.Method, req.URL)
	}

	// add cseq
	c.cseq++
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(c.cseq), 10)}

	// add user agent
	req.Header["User-Agent"] = base.HeaderValue{"gortsplib"}

	if c.conf.OnRequest != nil {
		c.conf.OnRequest(req)
	}

	c.nconn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	err := req.Write(c.bw)
	if err != nil {
		return nil, err
	}

	if req.SkipResponse {
		return nil, nil
	}

	// read the response and ignore interleaved frames in between;
	// interleaved frames are sent in two situations:
	// * when the server is v4lrtspserver, before the PLAY response
	// * when the stream is already playing
	var res base.Response
	c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	err = res.ReadIgnoreFrames(c.br, c.tcpFrameBuffer.Next())
	if err != nil {
		return nil, err
	}

	if c.conf.OnResponse != nil {
		c.conf.OnResponse(&res)
	}

	// get session from response
	if v, ok := res.Header["Session"]; ok {
		sx, err := headers.ReadSession(v)
		if err != nil {
			return nil, fmt.Errorf("unable to parse session header: %s", err)
		}
		c.session = sx.Session
	}

	// setup authentication
	if res.StatusCode == base.StatusUnauthorized && req.URL.User != nil && c.sender == nil {
		pass, _ := req.URL.User.Password()
		user := req.URL.User.Username()

		sender, err := auth.NewSender(res.Header["WWW-Authenticate"], user, pass)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		c.sender = sender

		// send request again
		return c.Do(req)
	}

	return &res, nil
}

// Options writes an OPTIONS request and reads a response.
func (c *ClientConn) Options(u *base.URL) (*base.Response, error) {
	err := c.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
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
		return res, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.getParameterSupported = func() bool {
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
func (c *ClientConn) Describe(u *base.URL) (Tracks, *base.Response, error) {
	err := c.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, nil, err
	}

	res, err := c.Do(&base.Request{
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
		if !c.conf.RedirectDisable &&
			res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {

			c.Close()

			u, err := base.ParseURL(res.Header["Location"][0])
			if err != nil {
				return nil, nil, err
			}

			nc, err := c.conf.Dial(u.Scheme, u.Host)
			if err != nil {
				return nil, nil, err
			}
			*c = *nc //nolint:govet

			_, err = c.Options(u)
			if err != nil {
				return nil, nil, err
			}

			return c.Describe(u)
		}

		return nil, res, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	payloadType, ok := res.Header["Content-Type"]
	if !ok || len(payloadType) != 1 {
		return nil, nil, fmt.Errorf("Content-Type not provided")
	}

	if payloadType[0] != "application/sdp" {
		return nil, nil, fmt.Errorf("wrong Content-Type, expected application/sdp")
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
func (c *ClientConn) Setup(mode headers.TransportMode, track *Track,
	rtpPort int, rtcpPort int) (*base.Response, error) {
	err := c.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	if mode == headers.TransportModeRecord && c.state != clientConnStatePreRecord {
		return nil, fmt.Errorf("cannot read and publish at the same time")
	}

	if mode == headers.TransportModePlay && c.state != clientConnStatePrePlay &&
		c.state != clientConnStateInitial {
		return nil, fmt.Errorf("cannot read and publish at the same time")
	}

	if c.streamURL != nil && *track.BaseURL != *c.streamURL {
		return nil, fmt.Errorf("cannot setup tracks with different base urls")
	}

	var rtpListener *clientConnUDPListener
	var rtcpListener *clientConnUDPListener

	// always use TCP if encrypted
	if c.isTLS {
		v := StreamProtocolTCP
		c.streamProtocol = &v
	}

	proto := func() StreamProtocol {
		// protocol set by previous Setup()
		if c.streamProtocol != nil {
			return *c.streamProtocol
		}

		// protocol set by conf
		if c.conf.StreamProtocol != nil {
			return *c.conf.StreamProtocol
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
			return nil, fmt.Errorf("rtpPort and rtcpPort must be both zero or non-zero")
		}

		if rtpPort != 0 && rtcpPort != (rtpPort+1) {
			return nil, fmt.Errorf("rtcpPort must be rtpPort + 1")
		}

		var err error
		rtpListener, rtcpListener, err = func() (*clientConnUDPListener, *clientConnUDPListener, error) {
			if rtpPort != 0 {
				rtpListener, err := newClientConnUDPListener(c, rtpPort)
				if err != nil {
					return nil, nil, err
				}

				rtcpListener, err := newClientConnUDPListener(c, rtcpPort)
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

				rtpListener, err := newClientConnUDPListener(c, rtpPort)
				if err != nil {
					continue
				}

				rtcpListener, err := newClientConnUDPListener(c, rtcpPort)
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
		th.InterleavedIds = &[2]int{(track.ID * 2), (track.ID * 2) + 1}
	}

	trackURL, err := track.URL()
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, err
	}

	res, err := c.Do(&base.Request{
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
			c.streamProtocol == nil &&
			c.conf.StreamProtocol == nil {

			v := StreamProtocolTCP
			c.streamProtocol = &v

			return c.Setup(headers.TransportModePlay, track, 0, 0)
		}

		return res, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	thRes, err := headers.ReadTransport(res.Header["Transport"])
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, fmt.Errorf("transport header: %s", err)
	}

	if proto == StreamProtocolUDP {
		if thRes.ServerPorts != nil {
			if (thRes.ServerPorts[0] == 0 && thRes.ServerPorts[1] != 0) ||
				(thRes.ServerPorts[0] != 0 && thRes.ServerPorts[1] == 0) {
				rtpListener.close()
				rtcpListener.close()
				return nil, fmt.Errorf("server ports must be both zero or both not zero")
			}
		}

		if !c.conf.AnyPortEnable {
			if thRes.ServerPorts == nil {
				rtpListener.close()
				rtcpListener.close()
				return nil, fmt.Errorf("server ports have not been provided. Use AnyPortEnable to communicate with this server")
			}

			if thRes.ServerPorts[0] == 0 && thRes.ServerPorts[1] == 0 {
				rtpListener.close()
				rtcpListener.close()
				return nil, fmt.Errorf("server ports have not been provided. Use AnyPortEnable to communicate with this server")
			}
		}

	} else {
		if thRes.InterleavedIds == nil ||
			(*thRes.InterleavedIds)[0] != (*th.InterleavedIds)[0] ||
			(*thRes.InterleavedIds)[1] != (*th.InterleavedIds)[1] {
			return nil, fmt.Errorf("transport header does not have interleaved ids %v (%s)",
				*th.InterleavedIds, res.Header["Transport"])
		}
	}

	clockRate, _ := track.ClockRate()

	if mode == headers.TransportModePlay {
		c.rtcpReceivers[track.ID] = rtcpreceiver.New(nil, clockRate)

		if proto == StreamProtocolUDP {
			v := time.Now().Unix()
			c.udpLastFrameTimes[track.ID] = &v
		}
	} else {
		c.rtcpSenders[track.ID] = rtcpsender.New(clockRate)
	}

	c.streamURL = track.BaseURL
	c.streamProtocol = &proto
	c.tracks = append(c.tracks, track)

	if proto == StreamProtocolUDP {
		rtpListener.remoteIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtpListener.remotePort = (*thRes.ServerPorts)[0]
		}
		rtpListener.trackID = track.ID
		rtpListener.streamType = StreamTypeRTP
		c.udpRTPListeners[track.ID] = rtpListener

		rtcpListener.remoteIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtcpListener.remotePort = (*thRes.ServerPorts)[1]
		}
		rtcpListener.trackID = track.ID
		rtcpListener.streamType = StreamTypeRTCP
		c.udpRTCPListeners[track.ID] = rtcpListener
	}

	if mode == headers.TransportModePlay {
		c.state = clientConnStatePrePlay
	} else {
		c.state = clientConnStatePreRecord
	}

	return res, nil
}

// Pause writes a PAUSE request and reads a Response.
// This can be called only after Play() or Record().
func (c *ClientConn) Pause() (*base.Response, error) {
	err := c.checkState(map[clientConnState]struct{}{
		clientConnStatePlay:   {},
		clientConnStateRecord: {},
	})
	if err != nil {
		return nil, err
	}

	close(c.backgroundTerminate)
	<-c.backgroundDone

	res, err := c.Do(&base.Request{
		Method: base.Pause,
		URL:    c.streamURL,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return res, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	switch c.state {
	case clientConnStatePlay:
		c.state = clientConnStatePrePlay
	case clientConnStateRecord:
		c.state = clientConnStatePreRecord
	}

	return res, nil
}
