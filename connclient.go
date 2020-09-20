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
	streamUrl         *url.URL
	streamProtocol    *StreamProtocol
	rtcpReceivers     map[int]*RtcpReceiver
	udpLastFrameTimes map[int]*int64
	udpRtpListeners   map[int]*connClientUDPListener
	udpRtcpListeners  map[int]*connClientUDPListener
	tcpFrames         *multiFrame
	playing           bool

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
	if c.playing {
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

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.nconn
}

// ReadFrame reads an InterleavedFrame.
func (c *ConnClient) ReadFrame() (*InterleavedFrame, error) {
	c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	frame := c.tcpFrames.next()
	err := frame.Read(c.br)
	if err != nil {
		return nil, err
	}

	c.rtcpReceivers[frame.TrackId].OnFrame(frame.StreamType, frame.Content)

	return frame, nil
}

func (c *ConnClient) readFrameOrResponse() (interface{}, error) {
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

// Do writes a Request and reads a Response.
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

	c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	res, err := ReadResponse(c.br)
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

// this can't be exported
// otherwise there's a race condition with the rtcp receiver report routine
func (c *ConnClient) writeFrame(frame *InterleavedFrame) error {
	c.nconn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.Write(c.bw)
}

// Options writes an OPTIONS request and reads a response, that contains
// the methods allowed by the server. Since this method is not implemented by
// every RTSP server, the function does not fail if the returned code is StatusNotFound.
func (c *ConnClient) Options(u *url.URL) (*Response, error) {
	if c.playing {
		return nil, fmt.Errorf("can't be called when playing")
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
		return nil, fmt.Errorf("OPTIONS: bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	return res, nil
}

// Describe writes a DESCRIBE request, that means that we want to obtain the SDP
// document that describes the tracks available in the given URL. It then
// reads a Response.
func (c *ConnClient) Describe(u *url.URL) (Tracks, *Response, error) {
	if c.playing {
		return nil, nil, fmt.Errorf("can't be called when playing")
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
		return nil, nil, fmt.Errorf("DESCRIBE: bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	contentType, ok := res.Header["Content-Type"]
	if !ok || len(contentType) != 1 {
		return nil, nil, fmt.Errorf("DESCRIBE: Content-Type not provided")
	}

	if contentType[0] != "application/sdp" {
		return nil, nil, fmt.Errorf("DESCRIBE: wrong Content-Type, expected application/sdp")
	}

	tracks, err := ReadTracks(res.Content)
	if err != nil {
		return nil, nil, err
	}

	return tracks, res, nil
}

// build an URL by merging baseUrl with the control attribute from track.Media
func (c *ConnClient) urlForTrack(baseUrl *url.URL, track *Track) *url.URL {
	// get control attribute
	control := func() string {
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

func (c *ConnClient) setup(u *url.URL, track *Track, ht *HeaderTransport) (*Response, error) {
	res, err := c.Do(&Request{
		Method: SETUP,
		Url:    c.urlForTrack(u, track),
		Header: Header{
			"Transport": ht.Write(),
		},
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != StatusOK {
		return nil, fmt.Errorf("SETUP: bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	return res, nil
}

// SetupUDP writes a SETUP request, that means that we want to read
// a given track with the UDP transport. It then reads a Response.
// If rtpPort and rtcpPort are zero, they will be chosen automatically.
func (c *ConnClient) SetupUDP(u *url.URL, track *Track, rtpPort int,
	rtcpPort int) (UDPReadFunc, UDPReadFunc, *Response, error) {
	if c.playing {
		return nil, nil, nil, fmt.Errorf("can't be called when playing")
	}

	if c.streamUrl != nil && *u != *c.streamUrl {
		fmt.Errorf("setup has already begun with another url")
	}

	if c.streamProtocol != nil && *c.streamProtocol != StreamProtocolUDP {
		return nil, nil, nil, fmt.Errorf("cannot setup tracks with different protocols")
	}

	if _, ok := c.rtcpReceivers[track.Id]; ok {
		return nil, nil, nil, fmt.Errorf("track has already been setup")
	}

	if (rtpPort == 0 && rtcpPort != 0) ||
		(rtpPort != 0 && rtcpPort == 0) {
		return nil, nil, nil, fmt.Errorf("rtpPort and rtcpPort must be both zero or non-zero")
	}

	if rtpPort != 0 && rtcpPort != (rtpPort+1) {
		return nil, nil, nil, fmt.Errorf("rtcpPort must be rtpPort + 1")
	}

	rtpListener, rtcpListener, err := func() (*connClientUDPListener, *connClientUDPListener, error) {
		if rtpPort != 0 {
			rtpListener, err := newConnClientUDPListener(c, rtpPort, track.Id, StreamTypeRtp)
			if err != nil {
				return nil, nil, err
			}

			rtcpListener, err := newConnClientUDPListener(c, rtcpPort, track.Id, StreamTypeRtcp)
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

				rtpListener, err := newConnClientUDPListener(c, rtpPort, track.Id, StreamTypeRtp)
				if err != nil {
					continue
				}

				rtcpListener, err := newConnClientUDPListener(c, rtcpPort, track.Id, StreamTypeRtcp)
				if err != nil {
					rtpListener.close()
					continue
				}

				return rtpListener, rtcpListener, nil
			}
		}
	}()
	if err != nil {
		return nil, nil, nil, err
	}

	res, err := c.setup(u, track, &HeaderTransport{
		Protocol: StreamProtocolUDP,
		Cast: func() *StreamCast {
			ret := StreamUnicast
			return &ret
		}(),
		ClientPorts: &[2]int{rtpPort, rtcpPort},
	})
	if err != nil {
		rtpListener.close()
		rtcpListener.close()
		return nil, nil, nil, err
	}

	th, err := ReadHeaderTransport(res.Header["Transport"])
	if err != nil {
		rtpListener.close()
		rtcpListener.close()
		return nil, nil, nil, fmt.Errorf("SETUP: transport header: %s", err)
	}

	if th.ServerPorts == nil {
		rtpListener.close()
		rtcpListener.close()
		return nil, nil, nil, fmt.Errorf("SETUP: server ports not provided")
	}

	c.streamUrl = u
	streamProtocol := StreamProtocolUDP
	c.streamProtocol = &streamProtocol
	c.rtcpReceivers[track.Id] = NewRtcpReceiver()

	v := time.Now().Unix()
	c.udpLastFrameTimes[track.Id] = &v

	rtpListener.publisherIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
	rtpListener.publisherPort = (*th.ServerPorts)[0]
	c.udpRtpListeners[track.Id] = rtpListener

	rtcpListener.publisherIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
	rtcpListener.publisherPort = (*th.ServerPorts)[1]
	c.udpRtcpListeners[track.Id] = rtcpListener

	return rtpListener.read, rtcpListener.read, res, nil
}

// SetupTCP writes a SETUP request, that means that we want to read
// a given track with the TCP transport. It then reads a Response.
func (c *ConnClient) SetupTCP(u *url.URL, track *Track) (*Response, error) {
	if c.playing {
		return nil, fmt.Errorf("can't be called when playing")
	}

	if c.streamUrl != nil && *u != *c.streamUrl {
		fmt.Errorf("setup has already begun with another url")
	}

	if c.streamProtocol != nil && *c.streamProtocol != StreamProtocolTCP {
		return nil, fmt.Errorf("cannot setup tracks with different protocols")
	}

	if _, ok := c.rtcpReceivers[track.Id]; ok {
		return nil, fmt.Errorf("track has already been setup")
	}

	interleavedIds := [2]int{(track.Id * 2), (track.Id * 2) + 1}
	res, err := c.setup(u, track, &HeaderTransport{
		Protocol: StreamProtocolTCP,
		Cast: func() *StreamCast {
			ret := StreamUnicast
			return &ret
		}(),
		InterleavedIds: &interleavedIds,
	})
	if err != nil {
		return nil, err
	}

	th, err := ReadHeaderTransport(res.Header["Transport"])
	if err != nil {
		return nil, fmt.Errorf("SETUP: transport header: %s", err)
	}

	if th.InterleavedIds == nil || (*th.InterleavedIds)[0] != interleavedIds[0] ||
		(*th.InterleavedIds)[1] != interleavedIds[1] {
		return nil, fmt.Errorf("SETUP: transport header does not have interleaved ids %v (%s)",
			interleavedIds, res.Header["Transport"])
	}

	c.streamUrl = u
	streamProtocol := StreamProtocolTCP
	c.streamProtocol = &streamProtocol
	c.rtcpReceivers[track.Id] = NewRtcpReceiver()

	return res, nil
}

// Play writes a PLAY request, that means that we want to start the stream.
// It then reads a Response. This function can be called only after SetupUDP()
// or SetupTCP().
func (c *ConnClient) Play(u *url.URL) (*Response, error) {
	if c.playing {
		return nil, fmt.Errorf("can't be called when playing")
	}

	if c.streamUrl == nil {
		return nil, fmt.Errorf("can be called only after a successful SetupUDP() or SetupTCP()")
	}

	if *u != *c.streamUrl {
		fmt.Errorf("must be called with the same url used for SetupUDP() or SetupTCP()")
	}

	res, err := func() (*Response, error) {
		if *c.streamProtocol == StreamProtocolUDP {
			res, err := c.Do(&Request{
				Method: PLAY,
				Url:    u,
			})
			if err != nil {
				return nil, err
			}

			return res, nil

		} else {
			_, err := c.Do(&Request{
				Method:       PLAY,
				Url:          u,
				SkipResponse: true,
			})
			if err != nil {
				return nil, err
			}

			c.tcpFrames = newMultiFrame(c.conf.ReadBufferCount, clientTCPFrameReadBufferSize)

			// v4lrtspserver sends frames before the response.
			// ignore them and wait for the response.
			for {
				recv, err := c.readFrameOrResponse()
				if err != nil {
					return nil, err
				}

				if res, ok := recv.(*Response); ok {
					return res, nil
				}
			}
		}
	}()
	if err != nil {
		return nil, err
	}

	if res.StatusCode != StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.playing = true

	// open the firewall by sending packets to every channel
	if *c.streamProtocol == StreamProtocolUDP {
		for trackId := range c.udpRtpListeners {
			c.udpRtpListeners[trackId].pc.WriteTo(
				[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
				&net.UDPAddr{
					IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
					Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
					Port: c.udpRtpListeners[trackId].publisherPort,
				})

			c.udpRtcpListeners[trackId].pc.WriteTo(
				[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
				&net.UDPAddr{
					IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
					Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
					Port: c.udpRtcpListeners[trackId].publisherPort,
				})
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
						c.udpRtcpListeners[trackId].pc.WriteTo(frame, &net.UDPAddr{
							IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
							Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
							Port: c.udpRtcpListeners[trackId].publisherPort,
						})

					} else {
						c.writeFrame(&InterleavedFrame{
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
// the TCP connection open through keepalives, and returns when the TCP
// connection closes.
func (c *ConnClient) LoopUDP(u *url.URL) error {
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
					Host:   u.Host,
					User:   u.User,
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
