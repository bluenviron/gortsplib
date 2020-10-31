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
	connClientStateReading
	connClientStatePublishing
)

// ConnClientConf allows to configure a ConnClient.
type ConnClientConf struct {
	// target address in format hostname:port
	// either Host or Conn must be non-null
	Host string

	// pre-existing TCP connection to wrap
	// either Host or Conn must be non-null
	Conn net.Conn

	// (optional) timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// (optional) timeout of write operations.
	// It defaults to 5 seconds
	WriteTimeout time.Duration

	// (optional) read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
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
	br                *bufio.Reader
	bw                *bufio.Writer
	session           string
	cseq              int
	auth              *auth.Client
	state             connClientState
	streamUrl         *url.URL
	streamProtocol    *StreamProtocol
	tracks            map[int]*Track
	rtcpReceivers     map[int]*rtcpreceiver.RtcpReceiver
	udpLastFrameTimes map[int]*int64
	udpRtpListeners   map[int]*connClientUDPListener
	udpRtcpListeners  map[int]*connClientUDPListener
	response          *base.Response
	frame             *base.InterleavedFrame
	tcpFrameBuffer    *multibuffer.MultiBuffer

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

	if conf.Host != "" && conf.Conn != nil {
		return nil, fmt.Errorf("Host and Conn can't be used together")
	}

	if conf.Conn == nil {
		if !strings.Contains(conf.Host, ":") {
			conf.Host += ":554"
		}

		var err error
		conf.Conn, err = conf.DialTimeout("tcp", conf.Host, conf.ReadTimeout)
		if err != nil {
			return nil, err
		}
	}

	return &ConnClient{
		conf:              conf,
		br:                bufio.NewReaderSize(conf.Conn, clientReadBufferSize),
		bw:                bufio.NewWriterSize(conf.Conn, clientWriteBufferSize),
		tracks:            make(map[int]*Track),
		rtcpReceivers:     make(map[int]*rtcpreceiver.RtcpReceiver),
		udpLastFrameTimes: make(map[int]*int64),
		udpRtpListeners:   make(map[int]*connClientUDPListener),
		udpRtcpListeners:  make(map[int]*connClientUDPListener),
		response:          &base.Response{},
		frame:             &base.InterleavedFrame{},
		tcpFrameBuffer:    multibuffer.New(conf.ReadBufferCount, clientTCPFrameReadBufferSize),
	}, nil
}

// Close closes all the ConnClient resources.
func (c *ConnClient) Close() error {
	if c.state == connClientStateReading {
		c.Do(&base.Request{
			Method:       base.TEARDOWN,
			URL:          c.streamUrl,
			SkipResponse: true,
		})
	}

	err := c.conf.Conn.Close()

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
	return c.conf.Conn
}

// Tracks returns all the tracks passed to SetupUDP() or SetupTCP().
func (c *ConnClient) Tracks() map[int]*Track {
	return c.tracks
}

func (c *ConnClient) readFrameTCPOrResponse() (interface{}, error) {
	c.frame.Content = c.tcpFrameBuffer.Next()

	c.conf.Conn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	return base.ReadInterleavedFrameOrResponse(c.frame, c.response, c.br)
}

// ReadFrameTCP reads an InterleavedFrame.
// This can't be used when publishing.
func (c *ConnClient) ReadFrameTCP() (int, StreamType, []byte, error) {
	c.frame.Content = c.tcpFrameBuffer.Next()

	c.conf.Conn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	err := c.frame.Read(c.br)
	if err != nil {
		return 0, 0, nil, err
	}

	c.rtcpReceivers[c.frame.TrackId].OnFrame(c.frame.StreamType, c.frame.Content)

	return c.frame.TrackId, c.frame.StreamType, c.frame.Content, nil
}

// ReadFrameUDP reads an UDP frame.
func (c *ConnClient) ReadFrameUDP(trackId int, streamType StreamType) ([]byte, error) {
	var buf []byte
	var err error
	if streamType == StreamTypeRtp {
		buf, err = c.udpRtpListeners[trackId].read()
		if err != nil {
			return nil, err
		}
	} else {
		buf, err = c.udpRtcpListeners[trackId].read()
		if err != nil {
			return nil, err
		}
	}

	atomic.StoreInt64(c.udpLastFrameTimes[trackId], time.Now().Unix())

	c.rtcpReceivers[trackId].OnFrame(streamType, buf)

	return buf, nil
}

// WriteFrameTCP writes an interleaved frame.
// this can't be used when reading.
func (c *ConnClient) WriteFrameTCP(trackId int, streamType StreamType, content []byte) error {
	frame := base.InterleavedFrame{
		TrackId:    trackId,
		StreamType: streamType,
		Content:    content,
	}

	c.conf.Conn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.Write(c.bw)
}

// WriteFrameUDP writes an UDP frame.
func (c *ConnClient) WriteFrameUDP(trackId int, streamType StreamType, content []byte) error {
	if streamType == StreamTypeRtp {
		return c.udpRtpListeners[trackId].write(content)
	}
	return c.udpRtcpListeners[trackId].write(content)
}

// Do writes a Request and reads a Response. Interleaved frames sent before the
// response are ignored.
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
		// remove credentials
		u := &url.URL{
			Scheme:   req.URL.Scheme,
			Host:     req.URL.Host,
			Path:     req.URL.Path,
			RawPath:  req.URL.RawPath,
			RawQuery: req.URL.RawQuery,
		}
		req.Header["Authorization"] = c.auth.GenerateHeader(req.Method, u)
	}

	// insert cseq
	c.cseq += 1
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(c.cseq), 10)}

	c.conf.Conn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
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
		pass, _ := req.URL.User.Password()
		auth, err := auth.NewClient(res.Header["WWW-Authenticate"], req.URL.User.Username(), pass)
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
func (c *ConnClient) Options(u *url.URL) (*base.Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	res, err := c.Do(&base.Request{
		Method: base.OPTIONS,
		URL: &url.URL{
			Scheme: "rtsp",
			Host:   u.Host,
			User:   u.User,
			// use the stream path, otherwise some cameras do not reply
			Path: u.Path,
		},
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK && res.StatusCode != base.StatusNotFound {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	return res, nil
}

// Describe writes a DESCRIBE request and reads a Response.
func (c *ConnClient) Describe(u *url.URL) (Tracks, *base.Response, error) {
	if c.state != connClientStateInitial {
		return nil, nil, fmt.Errorf("can't be called when reading or publishing")
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
func (c *ConnClient) urlForTrack(baseUrl *url.URL, mode TransportMode, track *Track) *url.URL {
	control := func() string {
		// if we're reading, get control from track ID
		if mode == TransportModeRecord {
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
			RawPath:  newUrl.RawPath,
			RawQuery: newUrl.RawQuery,
		}
	}

	// control attribute contains a relative path
	u := &url.URL{
		Scheme:   "rtsp",
		Host:     baseUrl.Host,
		User:     baseUrl.User,
		Path:     baseUrl.Path,
		RawPath:  baseUrl.RawPath,
		RawQuery: baseUrl.RawQuery,
	}
	base.URLAddControlPath(u, control)
	return u
}

func (c *ConnClient) setup(u *url.URL, mode TransportMode, track *Track, ht *headers.Transport) (*base.Response, error) {
	res, err := c.Do(&base.Request{
		Method: base.SETUP,
		URL:    c.urlForTrack(u, mode, track),
		Header: base.Header{
			"Transport": ht.Write(),
		},
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	return res, nil
}

// SetupUDP writes a SETUP request and reads a Response.
// If rtpPort and rtcpPort are zero, they are be chosen automatically.
func (c *ConnClient) SetupUDP(u *url.URL, mode TransportMode, track *Track, rtpPort int,
	rtcpPort int) (*base.Response, error) {
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

	res, err := c.setup(u, mode, track, &headers.Transport{
		Protocol: StreamProtocolUDP,
		Cast: func() *StreamCast {
			ret := StreamUnicast
			return &ret
		}(),
		ClientPorts: &[2]int{rtpPort, rtcpPort},
		Mode:        &mode,
	})
	if err != nil {
		rtpListener.close()
		rtcpListener.close()
		return nil, err
	}

	th, err := headers.ReadTransport(res.Header["Transport"])
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

	c.tracks[track.Id] = track

	if mode == TransportModePlay {
		c.rtcpReceivers[track.Id] = rtcpreceiver.New()

		v := time.Now().Unix()
		c.udpLastFrameTimes[track.Id] = &v
	}

	rtpListener.remoteIp = c.conf.Conn.RemoteAddr().(*net.TCPAddr).IP
	rtpListener.remoteZone = c.conf.Conn.RemoteAddr().(*net.TCPAddr).Zone
	rtpListener.remotePort = (*th.ServerPorts)[0]
	c.udpRtpListeners[track.Id] = rtpListener

	rtcpListener.remoteIp = c.conf.Conn.RemoteAddr().(*net.TCPAddr).IP
	rtcpListener.remoteZone = c.conf.Conn.RemoteAddr().(*net.TCPAddr).Zone
	rtcpListener.remotePort = (*th.ServerPorts)[1]
	c.udpRtcpListeners[track.Id] = rtcpListener

	return res, nil
}

// SetupTCP writes a SETUP request and reads a Response.
func (c *ConnClient) SetupTCP(u *url.URL, mode TransportMode, track *Track) (*base.Response, error) {
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
	res, err := c.setup(u, mode, track, &headers.Transport{
		Protocol: StreamProtocolTCP,
		Cast: func() *StreamCast {
			ret := StreamUnicast
			return &ret
		}(),
		InterleavedIds: &interleavedIds,
		Mode:           &mode,
	})
	if err != nil {
		return nil, err
	}

	th, err := headers.ReadTransport(res.Header["Transport"])
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

	c.tracks[track.Id] = track

	if mode == TransportModePlay {
		c.rtcpReceivers[track.Id] = rtcpreceiver.New()
	}

	return res, nil
}

// Play writes a PLAY request and reads a Response
// This function can be called only after SetupUDP() or SetupTCP().
func (c *ConnClient) Play(u *url.URL) (*base.Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	if c.streamUrl == nil {
		return nil, fmt.Errorf("can be called only after a successful SetupUDP() or SetupTCP()")
	}

	if *u != *c.streamUrl {
		fmt.Errorf("must be called with the same url used for SetupUDP() or SetupTCP()")
	}

	res, err := c.Do(&base.Request{
		Method: base.PLAY,
		URL:    u,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
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
						c.WriteFrameTCP(trackId, StreamTypeRtcp, frame)
					}
				}
			}
		}
	}()

	return res, nil
}

// LoopUDP must be called after Play() or Record(); it keeps
// the TCP connection open with keepalives, and returns when the TCP
// connection closes.
func (c *ConnClient) LoopUDP() error {
	if c.state != connClientStateReading && c.state != connClientStatePublishing {
		return fmt.Errorf("can be called only after a successful Play() or Record()")
	}

	if *c.streamProtocol != StreamProtocolUDP {
		return fmt.Errorf("stream protocol is not UDP")
	}

	if c.state == connClientStateReading {
		readDone := make(chan error)
		go func() {
			for {
				c.conf.Conn.SetReadDeadline(time.Now().Add(clientUDPKeepalivePeriod + c.conf.ReadTimeout))
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
				c.conf.Conn.Close()
				return err

			case <-keepaliveTicker.C:
				_, err := c.Do(&base.Request{
					Method: base.OPTIONS,
					URL: &url.URL{
						Scheme: "rtsp",
						Host:   c.streamUrl.Host,
						User:   c.streamUrl.User,
						// use the stream path, otherwise some cameras do not reply
						Path:    c.streamUrl.Path,
						RawPath: c.streamUrl.RawPath,
					},
					SkipResponse: true,
				})
				if err != nil {
					c.conf.Conn.Close()
					<-readDone
					return err
				}

			case <-checkStreamTicker.C:
				now := time.Now()

				for _, lastUnix := range c.udpLastFrameTimes {
					last := time.Unix(atomic.LoadInt64(lastUnix), 0)

					if now.Sub(last) >= c.conf.ReadTimeout {
						c.conf.Conn.Close()
						<-readDone
						return fmt.Errorf("no packets received recently (maybe there's a firewall/NAT in between)")
					}
				}
			}
		}
	}

	// connClientStatePublishing
	c.conf.Conn.SetReadDeadline(time.Time{}) // disable deadline
	var res base.Response
	return res.Read(c.br)
}

// Announce writes an ANNOUNCE request and reads a Response.
func (c *ConnClient) Announce(u *url.URL, tracks Tracks) (*base.Response, error) {
	if c.streamUrl != nil {
		fmt.Errorf("announce has already been sent with another url url")
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

	return res, nil
}

// Record writes a RECORD request and reads a Response.
func (c *ConnClient) Record(u *url.URL) (*base.Response, error) {
	if c.state != connClientStateInitial {
		return nil, fmt.Errorf("can't be called when reading or publishing")
	}

	if *u != *c.streamUrl {
		return nil, fmt.Errorf("must be called with the same url used for Announce()")
	}

	res, err := c.Do(&base.Request{
		Method: base.RECORD,
		URL:    u,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.state = connClientStatePublishing

	return nil, nil
}
