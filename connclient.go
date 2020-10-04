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
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
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
	connClientStateReading
	connClientStatePublishing
)

// ConnClientConf allows to configure a ConnClient.
type ConnClientConf struct {
	// target address in format hostname:port
	Host string

	// (optional) timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// (optional) timeout of write operations.
	// It defaults to 5 seconds
	WriteTimeout time.Duration

	// (optional) read buffer count.
	// If greater than 1, allows to pass frames to other routines than the one
	// that is reading frames.
	// It defaults to 1
	ReadBufferCount int

	// (optional) function used to initialize the TCP client.
	// It defaults to net.DialTimeout
	DialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)

	// (optional) function used to initialize UDP listeners.
	// It defaults to net.ListenPacket
	ListenPacket func(network, address string) (net.PacketConn, error)
}

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	conf              ConnClientConf
	nconn             net.Conn
	br                *bufio.Reader
	bw                *bufio.Writer
	session           string
	cseq              int
	auth              *authClient
	state             connClientState
	streamUrl         *url.URL
	streamProtocol    *StreamProtocol
	rtcpReceivers     map[int]*RtcpReceiver
	udpLastFrameTimes map[int]*int64
	udpRtpListeners   map[int]*connClientUDPListener
	udpRtcpListeners  map[int]*connClientUDPListener
	tcpFrames         *multiFrame

	receiverReportTerminate chan struct{}
	receiverReportDone      chan struct{}
}

// NewConnClient allocates a ConnClient. See ConnClientConf for the options.
func NewConnClient(conf ConnClientConf) (*ConnClient, error) {
	if conf.ReadTimeout == time.Duration(0) {
		conf.ReadTimeout = 10 * time.Second
	}
	if conf.WriteTimeout == time.Duration(0) {
		conf.WriteTimeout = 5 * time.Second
	}
	if conf.ReadBufferCount == 0 {
		conf.ReadBufferCount = 1
	}
	if conf.DialTimeout == nil {
		conf.DialTimeout = net.DialTimeout
	}
	if conf.ListenPacket == nil {
		conf.ListenPacket = net.ListenPacket
	}

	nconn, err := conf.DialTimeout("tcp", conf.Host, conf.ReadTimeout)
	if err != nil {
		return nil, err
	}

	return &ConnClient{
		conf:              conf,
		nconn:             nconn,
		br:                bufio.NewReaderSize(nconn, clientReadBufferSize),
		bw:                bufio.NewWriterSize(nconn, clientWriteBufferSize),
		rtcpReceivers:     make(map[int]*RtcpReceiver),
		udpLastFrameTimes: make(map[int]*int64),
		udpRtpListeners:   make(map[int]*connClientUDPListener),
		udpRtcpListeners:  make(map[int]*connClientUDPListener),
	}, nil
}

// Close closes all the ConnClient resources.
func (c *ConnClient) Close() error {
	if c.state == connClientStateReading {
		c.Do(&Request{
			Method:       TEARDOWN,
			Url:          c.streamUrl,
			SkipResponse: true,
		})
	}

	err := c.nconn.Close()

	if c.receiverReportTerminate != nil {
		close(c.receiverReportTerminate)
		<-c.receiverReportDone
	}

	for _, l := range c.udpRtpListeners {
		l.close()
	}

	for _, l := range c.udpRtcpListeners {
		l.close()
	}

	return err
}

// CloseUDPListeners closes any open UDP listener.
func (c *ConnClient) CloseUDPListeners() {
	for _, l := range c.udpRtpListeners {
		l.close()
	}

	for _, l := range c.udpRtcpListeners {
		l.close()
	}
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.nconn
}

func (c *ConnClient) readFrameTCPOrResponse() (interface{}, error) {
	c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	b, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	c.br.UnreadByte()

	if b == interleavedFrameMagicByte {
		frame := c.tcpFrames.next()
		err := frame.Read(c.br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	return ReadResponse(c.br)
}

// ReadFrameTCP reads an InterleavedFrame.
// This can't be used when recording.
func (c *ConnClient) ReadFrameTCP() (*InterleavedFrame, error) {
	c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	frame := c.tcpFrames.next()
	err := frame.Read(c.br)
	if err != nil {
		return nil, err
	}

	c.rtcpReceivers[frame.TrackId].OnFrame(frame.StreamType, frame.Content)

	return frame, nil
}

// ReadFrameUDP reads an UDP frame.
func (c *ConnClient) ReadFrameUDP(track *Track, streamType StreamType) ([]byte, error) {
	var buf []byte
	var err error
	if streamType == StreamTypeRtp {
		buf, err = c.udpRtpListeners[track.Id].read()
		if err != nil {
			return nil, err
		}
	} else {
		buf, err = c.udpRtcpListeners[track.Id].read()
		if err != nil {
			return nil, err
		}
	}

	atomic.StoreInt64(c.udpLastFrameTimes[track.Id], time.Now().Unix())

	c.rtcpReceivers[track.Id].OnFrame(streamType, buf)

	return buf, nil
}

// WriteFrameTCP writes an interleaved frame.
// this can't be used when playing.
func (c *ConnClient) WriteFrameTCP(frame *InterleavedFrame) error {
	c.nconn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.Write(c.bw)
}

// WriteFrameUDP writes an UDP frame.
func (c *ConnClient) WriteFrameUDP(track *Track, streamType StreamType, content []byte) error {
	if streamType == StreamTypeRtp {
		return c.udpRtpListeners[track.Id].write(content)
	}

	return c.udpRtcpListeners[track.Id].write(content)
}

// Do writes a Request and reads a Response. Interleaved frames sent before the
// response are ignored.
func (c *ConnClient) Do(req *Request) (*Response, error) {
	if req.Header == nil {
		req.Header = make(Header)
	}

	// insert session
	if c.session != "" {
		req.Header["Session"] = HeaderValue{c.session}
	}

	// insert auth
	if c.auth != nil {
		// remove credentials
		u := &url.URL{
			Scheme:   req.Url.Scheme,
			Host:     req.Url.Host,
			Path:     req.Url.Path,
			RawQuery: req.Url.RawQuery,
		}
		req.Header["Authorization"] = c.auth.GenerateHeader(req.Method, u)
	}

	// insert cseq
	c.cseq += 1
	req.Header["CSeq"] = HeaderValue{strconv.FormatInt(int64(c.cseq), 10)}

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
	res, err := func() (*Response, error) {
		for {
			recv, err := c.readFrameTCPOrResponse()
			if err != nil {
				return nil, err
			}

			if res, ok := recv.(*Response); ok {
				return res, nil
			}
		}
	}()
	if err != nil {
		return nil, err
	}

	// get session from response
	if v, ok := res.Header["Session"]; ok {
		sx, err := ReadHeaderSession(v)
		if err != nil {
			return nil, fmt.Errorf("unable to parse session header: %s", err)
		}
		c.session = sx.Session
	}

	// setup authentication
	if res.StatusCode == StatusUnauthorized && req.Url.User != nil && c.auth == nil {
		pass, _ := req.Url.User.Password()
		auth, err := newAuthClient(res.Header["WWW-Authenticate"], req.Url.User.Username(), pass)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		c.auth = auth

		// send request again
		return c.Do(req)
	}

	return res, nil
}

// Options writes an OPTIONS request and reads a response, that contains
// the methods allowed by the server. Since this method is not implemented by
// every RTSP server, the function does not fail if the returned code is StatusNotFound.
func (c *ConnClient) Options(u *url.URL) (*Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	res, err := c.Do(&Request{
		Method: OPTIONS,
		// strip path
		Url: &url.URL{
			Scheme: "rtsp",
			Host:   u.Host,
			User:   u.User,
			Path:   "/",
		},
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != StatusOK && res.StatusCode != StatusNotFound {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	return res, nil
}

// Describe writes a DESCRIBE request and reads a Response.
func (c *ConnClient) Describe(u *url.URL) (Tracks, *Response, error) {
	if c.state != connClientStateInitial {
		return nil, nil, fmt.Errorf("can't be called when reading or publishing")
	}

	res, err := c.Do(&Request{
		Method: DESCRIBE,
		Url:    u,
		Header: Header{
			"Accept": HeaderValue{"application/sdp"},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode != StatusOK {
		return nil, nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
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

// build an URL by merging baseUrl with the control attribute from track.Media
func (c *ConnClient) urlForTrack(baseUrl *url.URL, mode SetupMode, track *Track) *url.URL {
	control := func() string {
		// if we're recording, get control from track ID
		if mode == SetupModeRecord {
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
		newUrl, err := url.Parse(control)
		if err != nil {
			return baseUrl
		}

		return &url.URL{
			Scheme:   "rtsp",
			Host:     baseUrl.Host,
			User:     baseUrl.User,
			Path:     newUrl.Path,
			RawQuery: newUrl.RawQuery,
		}
	}

	// control attribute contains a relative path
	u := &url.URL{
		Scheme:   "rtsp",
		Host:     baseUrl.Host,
		User:     baseUrl.User,
		Path:     baseUrl.Path,
		RawQuery: baseUrl.RawQuery,
	}
	// insert the control attribute after the query, if present
	if u.RawQuery != "" {
		if !strings.HasSuffix(u.RawQuery, "/") {
			u.RawQuery += "/"
		}
		u.RawQuery += control
	} else {
		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
		u.Path += control
	}
	return u
}

func (c *ConnClient) setup(u *url.URL, mode SetupMode, track *Track, ht *HeaderTransport) (*Response, error) {
	res, err := c.Do(&Request{
		Method: SETUP,
		Url:    c.urlForTrack(u, mode, track),
		Header: Header{
			"Transport": ht.Write(),
		},
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	return res, nil
}

// SetupUDP writes a SETUP request and reads a Response.
// If rtpPort and rtcpPort are zero, they are be chosen automatically.
func (c *ConnClient) SetupUDP(u *url.URL, mode SetupMode, track *Track, rtpPort int,
	rtcpPort int) (*Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	if c.streamUrl != nil && *u != *c.streamUrl {
		fmt.Errorf("setup has already begun with another url")
	}

	if c.streamProtocol != nil && *c.streamProtocol != StreamProtocolUDP {
		return nil, fmt.Errorf("cannot setup tracks with different protocols")
	}

	if (rtpPort == 0 && rtcpPort != 0) ||
		(rtpPort != 0 && rtcpPort == 0) {
		return nil, fmt.Errorf("rtpPort and rtcpPort must be both zero or non-zero")
	}

	if rtpPort != 0 && rtcpPort != (rtpPort+1) {
		return nil, fmt.Errorf("rtcpPort must be rtpPort + 1")
	}

	rtpListener, rtcpListener, err := func() (*connClientUDPListener, *connClientUDPListener, error) {
		if rtpPort != 0 {
			rtpListener, err := newConnClientUDPListener(c.conf, rtpPort)
			if err != nil {
				return nil, nil, err
			}

			rtcpListener, err := newConnClientUDPListener(c.conf, rtcpPort)
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

				rtpListener, err := newConnClientUDPListener(c.conf, rtpPort)
				if err != nil {
					continue
				}

				rtcpListener, err := newConnClientUDPListener(c.conf, rtcpPort)
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

	res, err := c.setup(u, mode, track, &HeaderTransport{
		Protocol: StreamProtocolUDP,
		Cast: func() *StreamCast {
			ret := StreamUnicast
			return &ret
		}(),
		ClientPorts: &[2]int{rtpPort, rtcpPort},
		Mode: func() *string {
			var v string
			if mode == SetupModeRecord {
				v = "record"
			} else {
				v = "play"
			}
			return &v
		}(),
	})
	if err != nil {
		rtpListener.close()
		rtcpListener.close()
		return nil, err
	}

	th, err := ReadHeaderTransport(res.Header["Transport"])
	if err != nil {
		rtpListener.close()
		rtcpListener.close()
		return nil, fmt.Errorf("transport header: %s", err)
	}

	if th.ServerPorts == nil {
		rtpListener.close()
		rtcpListener.close()
		return nil, fmt.Errorf("server ports not provided")
	}

	c.streamUrl = u
	streamProtocol := StreamProtocolUDP
	c.streamProtocol = &streamProtocol

	if mode == SetupModePlay {
		c.rtcpReceivers[track.Id] = NewRtcpReceiver()

		v := time.Now().Unix()
		c.udpLastFrameTimes[track.Id] = &v
	}

	rtpListener.remoteIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
	rtpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
	rtpListener.remotePort = (*th.ServerPorts)[0]
	c.udpRtpListeners[track.Id] = rtpListener

	rtcpListener.remoteIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
	rtcpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
	rtcpListener.remotePort = (*th.ServerPorts)[1]
	c.udpRtcpListeners[track.Id] = rtcpListener

	return res, nil
}

// SetupTCP writes a SETUP request and reads a Response.
func (c *ConnClient) SetupTCP(u *url.URL, mode SetupMode, track *Track) (*Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	if c.streamUrl != nil && *u != *c.streamUrl {
		fmt.Errorf("setup has already begun with another url")
	}

	if c.streamProtocol != nil && *c.streamProtocol != StreamProtocolTCP {
		return nil, fmt.Errorf("cannot setup tracks with different protocols")
	}

	interleavedIds := [2]int{(track.Id * 2), (track.Id * 2) + 1}
	res, err := c.setup(u, mode, track, &HeaderTransport{
		Protocol: StreamProtocolTCP,
		Cast: func() *StreamCast {
			ret := StreamUnicast
			return &ret
		}(),
		InterleavedIds: &interleavedIds,
		Mode: func() *string {
			var v string
			if mode == SetupModeRecord {
				v = "record"
			} else {
				v = "play"
			}
			return &v
		}(),
	})
	if err != nil {
		return nil, err
	}

	th, err := ReadHeaderTransport(res.Header["Transport"])
	if err != nil {
		return nil, fmt.Errorf("transport header: %s", err)
	}

	if th.InterleavedIds == nil ||
		(*th.InterleavedIds)[0] != interleavedIds[0] ||
		(*th.InterleavedIds)[1] != interleavedIds[1] {
		return nil, fmt.Errorf("transport header does not have interleaved ids %v (%s)",
			interleavedIds, res.Header["Transport"])
	}

	c.streamUrl = u
	streamProtocol := StreamProtocolTCP
	c.streamProtocol = &streamProtocol

	if mode == SetupModePlay {
		c.rtcpReceivers[track.Id] = NewRtcpReceiver()
	}

	return res, nil
}

// Play writes a PLAY request and reads a Response
// This function can be called only after SetupUDP() or SetupTCP().
func (c *ConnClient) Play(u *url.URL) (*Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	if c.streamUrl == nil {
		return nil, fmt.Errorf("can be called only after a successful SetupUDP() or SetupTCP()")
	}

	if *u != *c.streamUrl {
		fmt.Errorf("must be called with the same url used for SetupUDP() or SetupTCP()")
	}

	if *c.streamProtocol == StreamProtocolTCP {
		c.tcpFrames = newMultiFrame(c.conf.ReadBufferCount, clientTCPFrameReadBufferSize)
	}

	res, err := c.Do(&Request{
		Method: PLAY,
		Url:    u,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.state = connClientStateReading

	// open the firewall by sending packets to the counterpart
	if *c.streamProtocol == StreamProtocolUDP {
		for trackId := range c.udpRtpListeners {
			c.udpRtpListeners[trackId].write(
				[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

			c.udpRtcpListeners[trackId].write(
				[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
		}
	}

	c.receiverReportTerminate = make(chan struct{})
	c.receiverReportDone = make(chan struct{})

	receiverReportTicker := time.NewTicker(clientReceiverReportPeriod)
	go func() {
		defer close(c.receiverReportDone)
		defer receiverReportTicker.Stop()

		for {
			select {
			case <-c.receiverReportTerminate:
				return

			case <-receiverReportTicker.C:
				for trackId := range c.rtcpReceivers {
					frame := c.rtcpReceivers[trackId].Report()

					if *c.streamProtocol == StreamProtocolUDP {
						c.udpRtcpListeners[trackId].write(frame)

					} else {
						c.WriteFrameTCP(&InterleavedFrame{
							TrackId:    trackId,
							StreamType: StreamTypeRtcp,
							Content:    frame,
						})
					}
				}
			}
		}
	}()

	return res, nil
}

// LoopUDP must be called after SetupUDP() and Play(); it keeps
// the TCP connection open with keepalives, and returns when the TCP
// connection closes.
func (c *ConnClient) LoopUDP() error {
	if c.state != connClientStateReading {
		return fmt.Errorf("can be called only after a successful Play()")
	}

	if *c.streamProtocol != StreamProtocolUDP {
		return fmt.Errorf("stream protocol is not UDP")
	}

	readDone := make(chan error)
	go func() {
		for {
			c.nconn.SetReadDeadline(time.Now().Add(clientUDPKeepalivePeriod + c.conf.ReadTimeout))
			_, err := ReadResponse(c.br)
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
			_, err := c.Do(&Request{
				Method: OPTIONS,
				Url: &url.URL{
					Scheme: "rtsp",
					Host:   c.streamUrl.Host,
					User:   c.streamUrl.User,
					Path:   "/",
				},
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

				if now.Sub(last) >= c.conf.ReadTimeout {
					c.nconn.Close()
					<-readDone
					return fmt.Errorf("no packets received recently (maybe there's a firewall/NAT in between)")
				}
			}
		}
	}
}

// Announce writes an ANNOUNCE request and reads a Response.
func (c *ConnClient) Announce(u *url.URL, tracks Tracks) (*Response, error) {
	if c.streamUrl != nil {
		fmt.Errorf("announce has already been sent with another url url")
	}

	res, err := c.Do(&Request{
		Method: ANNOUNCE,
		Url:    u,
		Header: Header{
			"Content-Type": HeaderValue{"application/sdp"},
		},
		Content: tracks.Write(),
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.streamUrl = u

	return res, nil
}

// Record writes a RECORD request and reads a Response.
func (c *ConnClient) Record(u *url.URL) (*Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	if *u != *c.streamUrl {
		return nil, fmt.Errorf("must be called with the same url used for Announce()")
	}

	res, err := c.Do(&Request{
		Method: RECORD,
		Url:    u,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.state = connClientStatePublishing

	return nil, nil
}
