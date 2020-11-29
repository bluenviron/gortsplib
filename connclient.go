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

type connClientState int

const (
	connClientStateInitial connClientState = iota
	connClientStatePrePlay
	connClientStatePlay
	connClientStatePreRecord
	connClientStateRecord
)

func (s connClientState) String() string {
	switch s {
	case connClientStateInitial:
		return "initial"
	case connClientStatePrePlay:
		return "prePlay"
	case connClientStatePlay:
		return "play"
	case connClientStatePreRecord:
		return "preRecord"
	case connClientStateRecord:
		return "record"
	}
	return "uknown"
}

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	d                     Dialer
	nconn                 net.Conn
	br                    *bufio.Reader
	bw                    *bufio.Writer
	session               string
	cseq                  int
	auth                  *auth.Client
	state                 connClientState
	streamUrl             *base.URL
	streamProtocol        *StreamProtocol
	tracks                Tracks
	udpRtpListeners       map[int]*connClientUDPListener
	udpRtcpListeners      map[int]*connClientUDPListener
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

// Close closes all the ConnClient resources.
func (c *ConnClient) Close() error {
	if c.state == connClientStatePlay || c.state == connClientStateRecord {
		close(c.backgroundTerminate)
		<-c.backgroundDone

		c.Do(&base.Request{
			Method:       base.TEARDOWN,
			URL:          c.streamUrl,
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

func (c *ConnClient) checkState(allowed map[connClientState]struct{}) error {
	if _, ok := allowed[c.state]; ok {
		return nil
	}

	var allowedList []connClientState
	for a := range allowed {
		allowedList = append(allowedList, a)
	}
	return fmt.Errorf("client must be in state %v, while is in state %v",
		allowedList, c.state)
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.nconn
}

// Tracks returns all the tracks that the connection is reading or publishing.
func (c *ConnClient) Tracks() Tracks {
	return c.tracks
}

func (c *ConnClient) readFrameTCPOrResponse() (interface{}, error) {
	c.nconn.SetReadDeadline(time.Now().Add(c.d.ReadTimeout))
	f := base.InterleavedFrame{
		Content: c.tcpFrameBuffer.Next(),
	}
	r := base.Response{}
	return base.ReadInterleavedFrameOrResponse(&f, &r, c.br)
}

// Do writes a Request and reads a Response.
// Interleaved frames received before the response are ignored.
func (c *ConnClient) Do(req *base.Request) (*base.Response, error) {
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
	c.cseq += 1
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(c.cseq), 10)}

	c.nconn.SetWriteDeadline(time.Now().Add(c.d.WriteTimeout))
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
func (c *ConnClient) Options(u *base.URL) (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStateInitial:   {},
		connClientStatePrePlay:   {},
		connClientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.OPTIONS,
		URL:    u,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return res, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.getParameterSupported = func() bool {
		pub, ok := res.Header["Public"]
		if !ok || len(pub) != 1 {
			return false
		}

		for _, m := range strings.Split(pub[0], ",") {
			if base.Method(m) == base.GET_PARAMETER {
				return true
			}
		}
		return false
	}()

	return res, nil
}

// Describe writes a DESCRIBE request and reads a Response.
func (c *ConnClient) Describe(u *base.URL) (Tracks, *base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStateInitial:   {},
		connClientStatePrePlay:   {},
		connClientStatePreRecord: {},
	})
	if err != nil {
		return nil, nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.DESCRIBE,
		URL:    u,
		Header: base.Header{
			"Accept": base.HeaderValue{"application/sdp"},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode != base.StatusOK {
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

	return tracks, res, nil
}

// build an URL by merging baseUrl with the control attribute from track.Media.
func (c *ConnClient) urlForTrack(baseUrl *base.URL, mode headers.TransportMode, track *Track) *base.URL {
	control := func() string {
		// if we're publishing, get control from track ID
		if mode == headers.TransportModeRecord {
			return "trackID=" + strconv.FormatInt(int64(track.Id), 10)
		}

		// otherwise, get from media attributes
		for _, attr := range track.Media.Attributes {
			if attr.Key == "control" {
				return attr.Value
			}
		}
		return ""
	}()

	// no control attribute, use base URL
	if control == "" {
		return baseUrl
	}

	// control attribute contains an absolute path
	if strings.HasPrefix(control, "rtsp://") {
		newUrl, err := base.ParseURL(control)
		if err != nil {
			return baseUrl
		}

		// copy host and credentials
		newUrl.Host = baseUrl.Host
		newUrl.User = baseUrl.User
		return newUrl
	}

	// control attribute contains a relative control attribute
	newUrl := baseUrl.Clone()
	newUrl.AddControlAttribute(control)
	return newUrl
}

// Setup writes a SETUP request and reads a Response.
// rtpPort and rtcpPort are used only if protocol is UDP.
// if rtpPort and rtcpPort are zero, they are chosen automatically.
func (c *ConnClient) Setup(u *base.URL, mode headers.TransportMode,
	track *Track, rtpPort int, rtcpPort int) (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStateInitial:   {},
		connClientStatePrePlay:   {},
		connClientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	if mode == headers.TransportModeRecord && c.state != connClientStatePreRecord {
		return nil, fmt.Errorf("cannot read and publish at the same time")
	}

	if mode == headers.TransportModePlay && c.state != connClientStatePrePlay &&
		c.state != connClientStateInitial {
		return nil, fmt.Errorf("cannot read and publish at the same time")
	}

	if c.streamUrl != nil && *u != *c.streamUrl {
		return nil, fmt.Errorf("setup has already begun with another url")
	}

	var rtpListener *connClientUDPListener
	var rtcpListener *connClientUDPListener

	proto := func() StreamProtocol {
		// protocol set by previous Setup()
		if c.streamProtocol != nil {
			return *c.streamProtocol
		}

		// protocol set by dialer
		if c.d.StreamProtocol != nil {
			return *c.d.StreamProtocol
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
		rtpListener, rtcpListener, err = func() (*connClientUDPListener, *connClientUDPListener, error) {
			if rtpPort != 0 {
				rtpListener, err := newConnClientUDPListener(c, rtpPort)
				if err != nil {
					return nil, nil, err
				}

				rtcpListener, err := newConnClientUDPListener(c, rtcpPort)
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

				rtpListener, err := newConnClientUDPListener(c, rtpPort)
				if err != nil {
					continue
				}

				rtcpListener, err := newConnClientUDPListener(c, rtcpPort)
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
		transport.InterleavedIds = &[2]int{(track.Id * 2), (track.Id * 2) + 1}
	}

	res, err := c.Do(&base.Request{
		Method: base.SETUP,
		URL:    c.urlForTrack(u, mode, track),
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
			c.d.StreamProtocol == nil {

			v := StreamProtocolTCP
			c.streamProtocol = &v

			return c.Setup(u, headers.TransportModePlay, track, 0, 0)
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
		c.rtcpReceivers[track.Id] = rtcpreceiver.New(nil, clockRate)

		if proto == StreamProtocolUDP {
			v := time.Now().Unix()
			c.udpLastFrameTimes[track.Id] = &v
		}
	} else {
		c.rtcpSenders[track.Id] = rtcpsender.New(clockRate)
	}

	c.streamUrl = u
	c.streamProtocol = &proto
	c.tracks = append(c.tracks, track)

	if proto == StreamProtocolUDP {
		rtpListener.remoteIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		rtpListener.remotePort = (*th.ServerPorts)[0]
		rtpListener.trackId = track.Id
		rtpListener.streamType = StreamTypeRtp
		c.udpRtpListeners[track.Id] = rtpListener

		rtcpListener.remoteIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		rtcpListener.remotePort = (*th.ServerPorts)[1]
		rtcpListener.trackId = track.Id
		rtcpListener.streamType = StreamTypeRtcp
		c.udpRtcpListeners[track.Id] = rtcpListener
	}

	if mode == headers.TransportModePlay {
		c.state = connClientStatePrePlay
	} else {
		c.state = connClientStatePreRecord
	}

	return res, nil
}

// Pause writes a PAUSE request and reads a Response.
// This can be called only after Play() or Record().
func (c *ConnClient) Pause() (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStatePlay:   {},
		connClientStateRecord: {},
	})
	if err != nil {
		return nil, err
	}

	close(c.backgroundTerminate)
	<-c.backgroundDone

	res, err := c.Do(&base.Request{
		Method: base.PAUSE,
		URL:    c.streamUrl,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return res, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	switch c.state {
	case connClientStatePlay:
		c.state = connClientStatePrePlay
	case connClientStateRecord:
		c.state = connClientStatePreRecord
	}

	return res, nil
}
