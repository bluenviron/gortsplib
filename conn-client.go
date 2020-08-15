/*
Package gortsplib is a RTSP 1.0 library for the Go programming language,
written for rtsp-simple-server.

Examples are available at https://github.com/aler9/gortsplib/tree/master/examples

*/
package gortsplib

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aler9/sdp/v3"
)

const (
	clientReadBufferSize       = 4096
	clientWriteBufferSize      = 4096
	clientReceiverReportPeriod = 10 * time.Second
	clientUdpCheckStreamPeriod = 5 * time.Second
	clientUdpKeepalivePeriod   = 30 * time.Second
	clientTcpReadBufferSize    = 128 * 1024
)

// Track is a track available in a certain URL.
type Track struct {
	// track id
	Id int

	// track codec and info in SDP format
	Media *sdp.MediaDescription
}

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

	// (optional) function used to initialize the TCP client.
	// It defaults to net.DialTimeout
	DialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)

	// (optional) function used to initialize UDP listeners.
	// It defaults to net.ListenPacket
	ListenPacket func(network, address string) (net.PacketConn, error)
}

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	conf           ConnClientConf
	nconn          net.Conn
	br             *bufio.Reader
	bw             *bufio.Writer
	session        string
	cseq           int
	auth           *authClient
	streamProtocol *StreamProtocol
	rtcpReceivers  map[int]*RtcpReceiver
	rtpListeners   map[int]*ConnClientUdpListener
	rtcpListeners  map[int]*ConnClientUdpListener

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
		conf:          conf,
		nconn:         nconn,
		br:            bufio.NewReaderSize(nconn, clientReadBufferSize),
		bw:            bufio.NewWriterSize(nconn, clientWriteBufferSize),
		rtcpReceivers: make(map[int]*RtcpReceiver),
		rtpListeners:  make(map[int]*ConnClientUdpListener),
		rtcpListeners: make(map[int]*ConnClientUdpListener),
	}, nil
}

// Close closes all the ConnClient resources.
func (c *ConnClient) Close() error {
	if c.receiverReportTerminate != nil {
		close(c.receiverReportTerminate)
		<-c.receiverReportDone
	}

	for _, rr := range c.rtcpReceivers {
		rr.Close()
	}

	for _, l := range c.rtpListeners {
		l.Close()
	}

	for _, l := range c.rtcpListeners {
		l.Close()
	}

	return c.nconn.Close()
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.nconn
}

// ReadFrame reads an InterleavedFrame.
func (c *ConnClient) ReadFrame(frame *InterleavedFrame) error {
	c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	err := frame.Read(c.br)
	if err != nil {
		return err
	}

	c.rtcpReceivers[frame.TrackId].OnFrame(frame.StreamType, frame.Content)
	return nil
}

// ReadFrameOrResponse reads an InterleavedFrame or a Response.
func (c *ConnClient) ReadFrameOrResponse(frame *InterleavedFrame) (interface{}, error) {
	c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	b, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	c.br.UnreadByte()

	if b == interleavedFrameMagicByte {
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

// WriteFrame writes an InterleavedFrame.
func (c *ConnClient) WriteFrame(frame *InterleavedFrame) error {
	c.nconn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.Write(c.bw)
}

// Options writes an OPTIONS request and reads a response, that contains
// the methods allowed by the server. Since this method is not implemented by
// every RTSP server, the function does not fail if the returned code is StatusNotFound.
func (c *ConnClient) Options(u *url.URL) (*Response, error) {
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
func (c *ConnClient) Describe(u *url.URL) ([]*Track, *Response, error) {
	res, err := c.Do(&Request{
		Method: DESCRIBE,
		Url:    u,
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

	sdpd := &sdp.SessionDescription{}
	err = sdpd.Unmarshal(res.Content)
	if err != nil {
		return nil, nil, err
	}

	tracks := make([]*Track, len(sdpd.MediaDescriptions))
	for i, media := range sdpd.MediaDescriptions {
		tracks[i] = &Track{
			Id:    i,
			Media: media,
		}
	}

	return tracks, res, nil
}

func (c *ConnClient) setup(u *url.URL, media *sdp.MediaDescription, transport []string) (*Response, error) {
	// build an URL with the control attribute from media
	u = func() *url.URL {
		control := func() string {
			for _, attr := range media.Attributes {
				if attr.Key == "control" {
					return attr.Value
				}
			}
			return ""
		}()

		// no control attribute, use original URL
		if control == "" {
			return u
		}

		// control attribute with absolute path
		if strings.HasPrefix(control, "rtsp://") {
			newu, err := url.Parse(control)
			if err != nil {
				return u
			}

			return &url.URL{
				Scheme:   "rtsp",
				Host:     u.Host,
				User:     u.User,
				Path:     newu.Path,
				RawQuery: newu.RawQuery,
			}
		}

		// control attribute with relative path
		return &url.URL{
			Scheme: "rtsp",
			Host:   u.Host,
			User:   u.User,
			Path: func() string {
				ret := u.Path
				if len(ret) == 0 || ret[len(ret)-1] != '/' {
					ret += "/"
				}
				ret += control
				return ret
			}(),
			RawQuery: u.RawQuery,
		}
	}()

	res, err := c.Do(&Request{
		Method: SETUP,
		Url:    u,
		Header: Header{
			"Transport": HeaderValue{strings.Join(transport, ";")},
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

// SetupUdp writes a SETUP request, that means that we want to read
// a given track with the UDP transport. It then reads a Response.
func (c *ConnClient) SetupUdp(u *url.URL, track *Track, rtpPort int,
	rtcpPort int) (*ConnClientUdpListener, *ConnClientUdpListener, *Response, error) {
	if c.streamProtocol != nil && *c.streamProtocol != StreamProtocolUdp {
		return nil, nil, nil, fmt.Errorf("cannot setup tracks with different protocols")
	}

	if _, ok := c.rtcpReceivers[track.Id]; ok {
		return nil, nil, nil, fmt.Errorf("track has already been setup")
	}

	rtpListener, err := newConnClientUdpListener(c, rtpPort, track.Id, StreamTypeRtp)
	if err != nil {
		return nil, nil, nil, err
	}

	rtcpListener, err := newConnClientUdpListener(c, rtcpPort, track.Id, StreamTypeRtcp)
	if err != nil {
		rtpListener.Close()
		return nil, nil, nil, err
	}

	res, err := c.setup(u, track.Media, []string{
		"RTP/AVP/UDP",
		"unicast",
		fmt.Sprintf("client_port=%d-%d", rtpPort, rtcpPort),
	})
	if err != nil {
		rtpListener.Close()
		rtcpListener.Close()
		return nil, nil, nil, err
	}

	th, err := ReadHeaderTransport(res.Header["Transport"])
	if err != nil {
		rtpListener.Close()
		rtcpListener.Close()
		return nil, nil, nil, fmt.Errorf("SETUP: transport header: %s", err)
	}

	rtpServerPort, rtcpServerPort := th.Ports("server_port")
	if rtpServerPort == 0 {
		rtpListener.Close()
		rtcpListener.Close()
		return nil, nil, nil, fmt.Errorf("SETUP: server ports not provided")
	}

	c.rtcpReceivers[track.Id] = NewRtcpReceiver()
	streamProtocol := StreamProtocolUdp
	c.streamProtocol = &streamProtocol

	rtpListener.publisherIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
	rtpListener.publisherPort = rtpServerPort
	c.rtpListeners[track.Id] = rtpListener

	rtcpListener.publisherIp = c.nconn.RemoteAddr().(*net.TCPAddr).IP
	rtcpListener.publisherPort = rtcpServerPort
	c.rtcpListeners[track.Id] = rtcpListener

	return rtpListener, rtcpListener, res, nil
}

// SetupTcp writes a SETUP request, that means that we want to read
// a given track with the TCP transport. It then reads a Response.
func (c *ConnClient) SetupTcp(u *url.URL, track *Track) (*Response, error) {
	if c.streamProtocol != nil && *c.streamProtocol != StreamProtocolTcp {
		return nil, fmt.Errorf("cannot setup tracks with different protocols")
	}

	if _, ok := c.rtcpReceivers[track.Id]; ok {
		return nil, fmt.Errorf("track has already been setup")
	}

	interleaved := fmt.Sprintf("interleaved=%d-%d", (track.Id * 2), (track.Id*2)+1)
	res, err := c.setup(u, track.Media, []string{
		"RTP/AVP/TCP",
		"unicast",
		interleaved,
	})
	if err != nil {
		return nil, err
	}

	th, err := ReadHeaderTransport(res.Header["Transport"])
	if err != nil {
		return nil, fmt.Errorf("SETUP: transport header: %s", err)
	}

	_, ok := th[interleaved]
	if !ok {
		return nil, fmt.Errorf("SETUP: transport header does not contain '%s' (%s)",
			interleaved, res.Header["Transport"])
	}

	c.rtcpReceivers[track.Id] = NewRtcpReceiver()
	streamProtocol := StreamProtocolTcp
	c.streamProtocol = &streamProtocol

	return res, nil
}

// Play writes a PLAY request, that means that we want to start the stream.
// It then reads a Response. This function can be called only after SetupUDP() or SetupTCP()
func (c *ConnClient) Play(u *url.URL) (*Response, error) {
	if c.streamProtocol == nil {
		return nil, fmt.Errorf("Play() can be called only after SetupUDP() or SetupTCP()")
	}

	_, err := c.Do(&Request{
		Method:       PLAY,
		Url:          u,
		SkipResponse: true,
	})
	if err != nil {
		return nil, err
	}

	res, err := func() (*Response, error) {
		frame := &InterleavedFrame{
			Content: make([]byte, 0, clientTcpReadBufferSize),
		}

		// v4lrtspserver sends frames before the response.
		// ignore them and wait for the response.
		for {
			frame.Content = frame.Content[:cap(frame.Content)]
			recv, err := c.ReadFrameOrResponse(frame)
			if err != nil {
				return nil, err
			}

			if res, ok := recv.(*Response); ok {
				if res.StatusCode != StatusOK {
					return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
				}

				return res, nil
			}
		}
	}()
	if err != nil {
		return nil, err
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

					if *c.streamProtocol == StreamProtocolUdp {
						c.rtcpListeners[trackId].pc.WriteTo(frame, &net.UDPAddr{
							IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
							Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
							Port: c.rtcpListeners[trackId].publisherPort,
						})

					} else {
						c.WriteFrame(&InterleavedFrame{
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
	// send a first keepalive before starting reading
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
		return err
	}

	readDone := make(chan error)
	go func() {
		for {
			c.nconn.SetReadDeadline(time.Now().Add(clientUdpKeepalivePeriod))
			_, err := ReadResponse(c.br)
			if err != nil {
				readDone <- err
				return
			}
		}
	}()

	keepaliveTicker := time.NewTicker(clientUdpKeepalivePeriod)
	defer keepaliveTicker.Stop()

	checkStreamTicker := time.NewTicker(clientUdpCheckStreamPeriod)
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
			for trackId := range c.rtcpReceivers {
				if time.Since(c.rtcpReceivers[trackId].LastFrameTime()) >= c.conf.ReadTimeout {
					c.nconn.Close()
					<-readDone
					return fmt.Errorf("stream is dead")
				}
			}
		}
	}
}
