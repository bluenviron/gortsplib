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
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/auth"
	"github.com/aler9/gortsplib/base"
	"github.com/aler9/gortsplib/headers"
	"github.com/aler9/gortsplib/multibuffer"
	"github.com/aler9/gortsplib/rtcpreceiver"
)

const (
	clientReadBufferSize         = 4096
	clientWriteBufferSize        = 4096
	clientReceiverReportPeriod   = 10 * time.Second
	clientUDPCheckStreamPeriod   = 5 * time.Second
	clientUDPKeepalivePeriod     = 30 * time.Second
	clientTCPFrameReadBufferSize = 128 * 1024
	clientUDPFrameReadBufferSize = 2048
)

type connClientState int

const (
	connClientStateInitial connClientState = iota
	connClientStatePrePlay
	connClientStatePlay
	connClientStatePreRecord
	connClientStateRecord
)

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
	rtcpReceivers         map[int]*rtcpreceiver.RtcpReceiver
	udpLastFrameTimes     map[int]*int64
	udpRtpListeners       map[int]*connClientUDPListener
	udpRtcpListeners      map[int]*connClientUDPListener
	response              *base.Response
	frame                 *base.InterleavedFrame
	tcpFrameBuffer        *multibuffer.MultiBuffer
	writeFrameFunc        func(trackId int, streamType StreamType, content []byte) error
	getParameterSupported bool

	reportWriterTerminate chan struct{}
	reportWriterDone      chan struct{}
}

// Close closes all the ConnClient resources.
func (c *ConnClient) Close() error {
	if c.state == connClientStatePlay {
		close(c.reportWriterTerminate)
		<-c.reportWriterDone

		c.Do(&base.Request{
			Method:       base.TEARDOWN,
			URL:          c.streamUrl,
			SkipResponse: true,
		})
	}

	err := c.nconn.Close()

	for _, l := range c.udpRtpListeners {
		l.close()
	}

	for _, l := range c.udpRtcpListeners {
		l.close()
	}

	return err
}

func (c *ConnClient) checkState(allowed map[connClientState]struct{}) error {
	if _, ok := allowed[c.state]; ok {
		return nil
	}

	return fmt.Errorf("client must be in state %v, while is in state %v",
		allowed, c.state)
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
	c.frame.Content = c.tcpFrameBuffer.Next()

	c.nconn.SetReadDeadline(time.Now().Add(c.d.ReadTimeout))
	return base.ReadInterleavedFrameOrResponse(c.frame, c.response, c.br)
}

// ReadFrameUDP reads an UDP frame.
func (c *ConnClient) ReadFrameUDP(trackId int, streamType StreamType) ([]byte, error) {
	var buf []byte
	var err error
	if streamType == StreamTypeRtp {
		buf, err = c.udpRtpListeners[trackId].read()
	} else {
		buf, err = c.udpRtcpListeners[trackId].read()
	}
	if err != nil {
		return nil, err
	}

	atomic.StoreInt64(c.udpLastFrameTimes[trackId], time.Now().Unix())

	c.rtcpReceivers[trackId].OnFrame(streamType, buf)

	return buf, nil
}

// ReadFrameTCP reads an InterleavedFrame.
// This can't be used when publishing.
func (c *ConnClient) ReadFrameTCP() (int, StreamType, []byte, error) {
	c.frame.Content = c.tcpFrameBuffer.Next()

	c.nconn.SetReadDeadline(time.Now().Add(c.d.ReadTimeout))
	err := c.frame.Read(c.br)
	if err != nil {
		return 0, 0, nil, err
	}

	c.rtcpReceivers[c.frame.TrackId].OnFrame(c.frame.StreamType, c.frame.Content)

	return c.frame.TrackId, c.frame.StreamType, c.frame.Content, nil
}

func (c *ConnClient) writeFrameUDP(trackId int, streamType StreamType, content []byte) error {
	if streamType == StreamTypeRtp {
		return c.udpRtpListeners[trackId].write(content)
	}
	return c.udpRtcpListeners[trackId].write(content)
}

func (c *ConnClient) writeFrameTCP(trackId int, streamType StreamType, content []byte) error {
	frame := base.InterleavedFrame{
		TrackId:    trackId,
		StreamType: streamType,
		Content:    content,
	}

	c.nconn.SetWriteDeadline(time.Now().Add(c.d.WriteTimeout))
	return frame.Write(c.bw)
}

// WriteFrame writes a frame.
// This can be used only after Record().
func (c *ConnClient) WriteFrame(trackId int, streamType StreamType, content []byte) error {
	return c.writeFrameFunc(trackId, streamType, content)
}

// Do writes a Request and reads a Response.
// Interleaved frames sent before the response are ignored.
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
// Since this method is not implemented by every RTSP server, the function
// does not fail if the returned code is StatusNotFound.
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

	if res.StatusCode != base.StatusOK && res.StatusCode != base.StatusNotFound {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
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

	switch res.StatusCode {
	case base.StatusOK:
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

	case base.StatusMovedPermanently, base.StatusFound,
		base.StatusSeeOther, base.StatusNotModified, base.StatusUseProxy:
		location, ok := res.Header["Location"]
		if !ok || len(location) != 1 {
			return nil, nil, fmt.Errorf("Location not provided")
		}

		return nil, res, nil

	default:
		return nil, nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}
}

// build an URL by merging baseUrl with the control attribute from track.Media.
func (c *ConnClient) urlForTrack(baseUrl *base.URL, mode headers.TransportMode, track *Track) *base.URL {
	control := func() string {
		// if we're reading, get control from track ID
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

	// control attribute contains a control attribute
	newUrl := baseUrl.Clone()
	newUrl.AddControlAttribute(control)
	return newUrl
}

// Setup writes a SETUP request and reads a Response.
// rtpPort and rtcpPort are used only if protocol is UDP.
// if rtpPort and rtcpPort are zero, they are chosen automatically.
func (c *ConnClient) Setup(u *base.URL, mode headers.TransportMode, proto base.StreamProtocol,
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

	if c.streamProtocol != nil && *c.streamProtocol != proto {
		return nil, fmt.Errorf("cannot setup tracks with different protocols")
	}

	var rtpListener *connClientUDPListener
	var rtcpListener *connClientUDPListener

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
				rtpListener, err := newConnClientUDPListener(c.d, rtpPort)
				if err != nil {
					return nil, nil, err
				}

				rtcpListener, err := newConnClientUDPListener(c.d, rtcpPort)
				if err != nil {
					rtpListener.close()
					return nil, nil, err
				}

				return rtpListener, rtcpListener, nil

			} else {
				for {
					// choose two consecutive ports in range 65535-10000
					// rtp must be even and rtcp odd
					rtpPort = (rand.Intn((65535-10000)/2) * 2) + 10000
					rtcpPort = rtpPort + 1

					rtpListener, err := newConnClientUDPListener(c.d, rtpPort)
					if err != nil {
						continue
					}

					rtcpListener, err := newConnClientUDPListener(c.d, rtcpPort)
					if err != nil {
						rtpListener.close()
						continue
					}

					return rtpListener, rtcpListener, nil
				}
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
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
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

	c.streamUrl = u
	streamProtocol := proto
	c.streamProtocol = &streamProtocol

	c.tracks = append(c.tracks, track)

	if mode == headers.TransportModePlay {
		c.rtcpReceivers[track.Id] = rtcpreceiver.New()

		if proto == StreamProtocolUDP {
			v := time.Now().Unix()
			c.udpLastFrameTimes[track.Id] = &v
		}
	}

	if proto == StreamProtocolUDP {
		rtpListener.remoteIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		rtpListener.remotePort = (*th.ServerPorts)[0]
		c.udpRtpListeners[track.Id] = rtpListener

		rtcpListener.remoteIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		rtcpListener.remotePort = (*th.ServerPorts)[1]
		c.udpRtcpListeners[track.Id] = rtcpListener
	}

	if mode == headers.TransportModePlay {
		c.state = connClientStatePrePlay
	} else {
		c.state = connClientStatePreRecord
	}

	return res, nil
}

// Play writes a PLAY request and reads a Response.
// This can be called only after Setup().
func (c *ConnClient) Play() (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.PLAY,
		URL:    c.streamUrl,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	if *c.streamProtocol == StreamProtocolUDP {
		c.writeFrameFunc = c.writeFrameUDP
	} else {
		c.writeFrameFunc = c.writeFrameTCP
	}

	c.state = connClientStatePlay

	// open the firewall by sending packets to the counterpart
	if *c.streamProtocol == StreamProtocolUDP {
		for trackId := range c.udpRtpListeners {
			c.WriteFrame(trackId, StreamTypeRtp,
				[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

			c.WriteFrame(trackId, StreamTypeRtcp,
				[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
		}
	}

	c.reportWriterTerminate = make(chan struct{})
	c.reportWriterDone = make(chan struct{})

	go func() {
		defer close(c.reportWriterDone)

		reportWriterTicker := time.NewTicker(clientReceiverReportPeriod)
		defer reportWriterTicker.Stop()

		for {
			select {
			case <-c.reportWriterTerminate:
				return

			case <-reportWriterTicker.C:
				for trackId := range c.rtcpReceivers {
					frame := c.rtcpReceivers[trackId].Report()
					c.WriteFrame(trackId, StreamTypeRtcp, frame)
				}
			}
		}
	}()

	return res, nil
}

// Announce writes an ANNOUNCE request and reads a Response.
func (c *ConnClient) Announce(u *base.URL, tracks Tracks) (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStateInitial: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.ANNOUNCE,
		URL:    u,
		Header: base.Header{
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Content: tracks.Write(),
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.streamUrl = u
	c.state = connClientStatePreRecord

	return res, nil
}

// Record writes a RECORD request and reads a Response.
// This can be called only after Announce() and Setup().
func (c *ConnClient) Record() (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.RECORD,
		URL:    c.streamUrl,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	if *c.streamProtocol == StreamProtocolUDP {
		c.writeFrameFunc = c.writeFrameUDP
	} else {
		c.writeFrameFunc = c.writeFrameTCP
	}

	c.state = connClientStateRecord

	return nil, nil
}

// LoopUDP must be called after Play() or Record(); it keeps
// the TCP connection open with keepalives, and returns when the TCP
// connection closes.
func (c *ConnClient) LoopUDP() error {
	err := c.checkState(map[connClientState]struct{}{
		connClientStatePlay:   {},
		connClientStateRecord: {},
	})
	if err != nil {
		return err
	}

	if *c.streamProtocol != StreamProtocolUDP {
		return fmt.Errorf("stream protocol is not UDP")
	}

	if c.state == connClientStatePlay {
		readDone := make(chan error)
		go func() {
			for {
				c.nconn.SetReadDeadline(time.Now().Add(clientUDPKeepalivePeriod + c.d.ReadTimeout))
				var res base.Response
				err := res.Read(c.br)
				if err != nil {
					readDone <- err
					return
				}
			}
		}()

		keepaliveTicker := time.NewTicker(clientUDPKeepalivePeriod)
		defer keepaliveTicker.Stop()

		checkStreamTicker := time.NewTicker(clientUDPCheckStreamPeriod)
		defer checkStreamTicker.Stop()

		for {
			select {
			case err := <-readDone:
				c.nconn.Close()
				return err

			case <-keepaliveTicker.C:
				_, err := c.Do(&base.Request{
					Method: func() base.Method {
						// the vlc integrated rtsp server requires GET_PARAMETER
						if c.getParameterSupported {
							return base.GET_PARAMETER
						}
						return base.OPTIONS
					}(),
					// use the stream path, otherwise some cameras do not reply
					URL:          c.streamUrl,
					SkipResponse: true,
				})
				if err != nil {
					c.nconn.Close()
					<-readDone
					return err
				}

			case <-checkStreamTicker.C:
				now := time.Now()

				for _, lastUnix := range c.udpLastFrameTimes {
					last := time.Unix(atomic.LoadInt64(lastUnix), 0)

					if now.Sub(last) >= c.d.ReadTimeout {
						c.nconn.Close()
						<-readDone
						return fmt.Errorf("no packets received recently (maybe there's a firewall/NAT in between)")
					}
				}
			}
		}
	}

	// connClientStateRecord
	c.nconn.SetReadDeadline(time.Time{}) // disable deadline
	var res base.Response
	return res.Read(c.br)
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

	if c.state == connClientStatePlay {
		close(c.reportWriterTerminate)
		<-c.reportWriterDone
	}

	res, err := c.Do(&base.Request{
		Method: base.PAUSE,
		URL:    c.streamUrl,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	if c.state == connClientStatePlay {
		c.state = connClientStatePrePlay
	} else {
		c.state = connClientStatePreRecord
	}

	return res, nil
}
