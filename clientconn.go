/*
Package gortsplib is a RTSP 1.0 library for the Go programming language,
written for rtsp-simple-server.

Examples are available at https://github.com/aler9/gortsplib/tree/master/examples

*/
package gortsplib

import (
	"bufio"
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
	clientReadBufferSize         = 4096
	clientWriteBufferSize        = 4096
	clientReceiverReportPeriod   = 10 * time.Second
	clientSenderReportPeriod     = 10 * time.Second
	clientUDPCheckStreamPeriod   = 5 * time.Second
	clientUDPKeepalivePeriod     = 30 * time.Second
	clientTCPFrameReadBufferSize = 128 * 1024
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
	return "uknown"
}

// ClientConn is a client-side RTSP connection.
type ClientConn struct {
	c                     ClientConf
	nconn                 net.Conn
	br                    *bufio.Reader
	bw                    *bufio.Writer
	session               string
	cseq                  int
	auth                  *auth.Client
	state                 clientConnState
	streamURL             *base.URL
	streamProtocol        *StreamProtocol
	tracks                Tracks
	udpRtpListeners       map[int]*clientConnUDPListener
	udpRtcpListeners      map[int]*clientConnUDPListener
	getParameterSupported bool

	// read only
	rtcpReceivers     map[int]*rtcpreceiver.RtcpReceiver
	udpLastFrameTimes map[int]*int64
	tcpFrameBuffer    *multibuffer.MultiBuffer
	readCB            func(int, StreamType, []byte)

	// publish only
	rtcpSenders       map[int]*rtcpsender.RtcpSender
	publishError      error
	publishWriteMutex sync.RWMutex
	publishOpen       bool

	// in
	backgroundTerminate chan struct{}

	// out
	backgroundDone chan struct{}
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

	for _, l := range c.udpRtpListeners {
		l.close()
	}

	for _, l := range c.udpRtcpListeners {
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
	return fmt.Errorf("client must be in state %v, while is in state %v",
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

func (c *ClientConn) readFrameTCPOrResponse() (interface{}, error) {
	c.nconn.SetReadDeadline(time.Now().Add(c.c.ReadTimeout))
	f := base.InterleavedFrame{
		Content: c.tcpFrameBuffer.Next(),
	}
	r := base.Response{}
	return base.ReadInterleavedFrameOrResponse(&f, &r, c.br)
}

// Do writes a Request and reads a Response.
// Interleaved frames received before the response are ignored.
func (c *ClientConn) Do(req *base.Request) (*base.Response, error) {
	if req.Header == nil {
		req.Header = make(base.Header)
	}

	// insert session
	if c.session != "" {
		req.Header["Session"] = base.HeaderValue{c.session}
	}

	// insert auth
	if c.auth != nil {
		req.Header["Authorization"] = c.auth.GenerateHeader(req.Method, req.URL)
	}

	// insert cseq
	c.cseq++
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(c.cseq), 10)}

	c.nconn.SetWriteDeadline(time.Now().Add(c.c.WriteTimeout))
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
	res, err := func() (*base.Response, error) {
		for {
			recv, err := c.readFrameTCPOrResponse()
			if err != nil {
				return nil, err
			}

			if res, ok := recv.(*base.Response); ok {
				return res, nil
			}
		}
	}()
	if err != nil {
		return nil, err
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
	if res.StatusCode == base.StatusUnauthorized && req.URL.User != nil && c.auth == nil {
		auth, err := auth.NewClient(res.Header["WWW-Authenticate"], req.URL.User)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		c.auth = auth

		// send request again
		return c.Do(req)
	}

	return res, nil
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
		if !c.c.RedirectDisable &&
			res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {

			c.Close()

			u, err := base.ParseURL(res.Header["Location"][0])
			if err != nil {
				return nil, nil, err
			}

			nc, err := c.c.Dial(u.Host)
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

	contentType, ok := res.Header["Content-Type"]
	if !ok || len(contentType) != 1 {
		return nil, nil, fmt.Errorf("Content-Type not provided")
	}

	if contentType[0] != "application/sdp" {
		return nil, nil, fmt.Errorf("wrong Content-Type, expected application/sdp")
	}

	tracks, err := ReadTracks(res.Content)
	if err != nil {
		return nil, nil, err
	}

	for _, t := range tracks {
		t.BaseURL = u
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

	proto := func() StreamProtocol {
		// protocol set by previous Setup()
		if c.streamProtocol != nil {
			return *c.streamProtocol
		}

		// protocol set by conf
		if c.c.StreamProtocol != nil {
			return *c.c.StreamProtocol
		}

		// try udp
		return StreamProtocolUDP
	}()

	transport := &headers.Transport{
		Protocol: proto,
		Delivery: func() *base.StreamDelivery {
			ret := base.StreamDeliveryUnicast
			return &ret
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

		transport.ClientPorts = &[2]int{rtpPort, rtcpPort}

	} else {
		transport.InterleavedIds = &[2]int{(track.ID * 2), (track.ID * 2) + 1}
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
			"Transport": transport.Write(),
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
			c.c.StreamProtocol == nil {

			v := StreamProtocolTCP
			c.streamProtocol = &v

			return c.Setup(headers.TransportModePlay, track, 0, 0)
		}

		return res, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	th, err := headers.ReadTransport(res.Header["Transport"])
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, fmt.Errorf("transport header: %s", err)
	}

	if proto == StreamProtocolUDP {
		if th.ServerPorts == nil {
			rtpListener.close()
			rtcpListener.close()
			return nil, fmt.Errorf("server ports not provided")
		}

	} else {
		if th.InterleavedIds == nil ||
			(*th.InterleavedIds)[0] != (*transport.InterleavedIds)[0] ||
			(*th.InterleavedIds)[1] != (*transport.InterleavedIds)[1] {
			return nil, fmt.Errorf("transport header does not have interleaved ids %v (%s)",
				*transport.InterleavedIds, res.Header["Transport"])
		}
	}

	clockRate, err := track.ClockRate()
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, fmt.Errorf("unable to get the track clock rate: %s", err)
	}

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
		rtpListener.remotePort = (*th.ServerPorts)[0]
		rtpListener.trackID = track.ID
		rtpListener.streamType = StreamTypeRtp
		c.udpRtpListeners[track.ID] = rtpListener

		rtcpListener.remoteIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		rtcpListener.remotePort = (*th.ServerPorts)[1]
		rtcpListener.trackID = track.ID
		rtcpListener.streamType = StreamTypeRtcp
		c.udpRtcpListeners[track.ID] = rtcpListener
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
