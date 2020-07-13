package gortsplib

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pion/sdp"
)

const (
	clientReadBufferSize  = 4096
	clientWriteBufferSize = 4096
)

// ConnClientConf allows to configure a ConnClient.
type ConnClientConf struct {
	// pre-existing TCP connection that will be wrapped
	Conn net.Conn

	// (optional) timeout for read requests.
	// It defaults to 5 seconds
	ReadTimeout time.Duration

	// (optional) timeout for write requests.
	// It defaults to 5 seconds
	WriteTimeout time.Duration
}

// ConnClient is a client-side RTSP connection.
type ConnClient struct {
	conf    ConnClientConf
	br      *bufio.Reader
	bw      *bufio.Writer
	session string
	curCSeq int
	auth    *AuthClient
}

// NewConnClient allocates a ConnClient. See ConnClientConf for the options.
func NewConnClient(conf ConnClientConf) (*ConnClient, error) {
	if conf.ReadTimeout == time.Duration(0) {
		conf.ReadTimeout = 5 * time.Second
	}
	if conf.WriteTimeout == time.Duration(0) {
		conf.WriteTimeout = 5 * time.Second
	}

	return &ConnClient{
		conf: conf,
		br:   bufio.NewReaderSize(conf.Conn, clientReadBufferSize),
		bw:   bufio.NewWriterSize(conf.Conn, clientWriteBufferSize),
	}, nil
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.conf.Conn
}

// ReadFrame reads an InterleavedFrame.
func (c *ConnClient) ReadFrame(frame *InterleavedFrame) error {
	return frame.read(c.br)
}

func (c *ConnClient) readFrameOrResponse(frame *InterleavedFrame) (interface{}, error) {
	c.conf.Conn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	b, err := c.br.ReadByte()
	if err != nil {
		return nil, err
	}
	c.br.UnreadByte()

	if b == interleavedFrameMagicByte {
		err := frame.read(c.br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	return readResponse(c.br)
}

// Do writes a Request and reads a Response.
func (c *ConnClient) Do(req *Request) (*Response, error) {
	err := c.writeRequest(req)
	if err != nil {
		return nil, err
	}

	c.conf.Conn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	res, err := readResponse(c.br)
	if err != nil {
		return nil, err
	}

	// get session from response
	if sxRaw, ok := res.Header["Session"]; ok && len(sxRaw) == 1 {
		sx, err := ReadHeaderSession(sxRaw[0])
		if err != nil {
			return nil, fmt.Errorf("unable to parse session header: %s", err)
		}
		c.session = sx.Session
	}

	// setup authentication
	if res.StatusCode == StatusUnauthorized && req.Url.User != nil && c.auth == nil {
		pass, _ := req.Url.User.Password()
		auth, err := NewAuthClient(res.Header["WWW-Authenticate"], req.Url.User.Username(), pass)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		c.auth = auth

		// send request again
		return c.Do(req)
	}

	return res, nil
}

func (c *ConnClient) writeRequest(req *Request) error {
	if req.Header == nil {
		req.Header = make(Header)
	}

	// insert session
	if c.session != "" {
		req.Header["Session"] = []string{c.session}
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
	c.curCSeq += 1
	req.Header["CSeq"] = []string{strconv.FormatInt(int64(c.curCSeq), 10)}

	c.conf.Conn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return req.write(c.bw)
}

// WriteFrame writes an InterleavedFrame.
func (c *ConnClient) WriteFrame(frame *InterleavedFrame) error {
	c.conf.Conn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.write(c.bw)
}

// Options writes an OPTIONS request and reads a response, that contains
// the methods allowed by the server. Since this method is not implemented by
// every RTSP server, the function does not fail if the returned code is StatusNotFound.
func (c *ConnClient) Options(u *url.URL) (*Response, error) {
	// strip path
	u = &url.URL{
		Scheme: "rtsp",
		Host:   u.Host,
		User:   u.User,
		Path:   "/",
	}

	res, err := c.Do(&Request{
		Method: OPTIONS,
		Url:    u,
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
// document that describes the stream available in the given url. It then
// reads a Response.
func (c *ConnClient) Describe(u *url.URL) (*sdp.SessionDescription, *Response, error) {
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
	err = sdpd.Unmarshal(string(res.Content))
	if err != nil {
		return nil, nil, err
	}

	return sdpd, res, nil
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

		// no control attribute, use original url
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
			"Transport": []string{strings.Join(transport, ";")},
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
// a track with given media, id and the UDP transport. It then reads a Response.
func (c *ConnClient) SetupUdp(u *url.URL, media *sdp.MediaDescription,
	rtpPort int, rtcpPort int) (int, int, *Response, error) {

	res, err := c.setup(u, media, []string{
		"RTP/AVP/UDP",
		"unicast",
		fmt.Sprintf("client_port=%d-%d", rtpPort, rtcpPort),
	})
	if err != nil {
		return 0, 0, nil, err
	}

	tsRaw, ok := res.Header["Transport"]
	if !ok || len(tsRaw) != 1 {
		return 0, 0, nil, fmt.Errorf("SETUP: transport header not provided")
	}

	th := ReadHeaderTransport(tsRaw[0])
	rtpServerPort, rtcpServerPort := th.GetPorts("server_port")
	if rtpServerPort == 0 {
		return 0, 0, nil, fmt.Errorf("SETUP: server ports not provided")
	}

	return rtpServerPort, rtcpServerPort, res, nil
}

// SetupTcp writes a SETUP request, that means that we want to read
// a track with given media, given id and the TCP transport. It then reads a Response.
func (c *ConnClient) SetupTcp(u *url.URL, media *sdp.MediaDescription, trackId int) (*Response, error) {
	interleaved := fmt.Sprintf("interleaved=%d-%d", (trackId * 2), (trackId*2)+1)

	res, err := c.setup(u, media, []string{
		"RTP/AVP/TCP",
		"unicast",
		interleaved,
	})
	if err != nil {
		return nil, err
	}

	tsRaw, ok := res.Header["Transport"]
	if !ok || len(tsRaw) != 1 {
		return nil, fmt.Errorf("SETUP: transport header not provided")
	}
	th := ReadHeaderTransport(tsRaw[0])

	_, ok = th[interleaved]
	if !ok {
		return nil, fmt.Errorf("SETUP: transport header does not have %s (%s)", interleaved, tsRaw[0])
	}

	return res, nil
}

// Play writes a PLAY request, that means that we want to start the
// stream. It then reads a Response.
func (c *ConnClient) Play(u *url.URL) (*Response, error) {
	err := c.writeRequest(&Request{
		Method: PLAY,
		Url:    u,
	})
	if err != nil {
		return nil, err
	}

	frame := &InterleavedFrame{
		Content: make([]byte, 512*1024),
	}

	// v4lrtspserver sends frames before the response.
	// ignore them and wait for the response.
	for {
		recv, err := c.readFrameOrResponse(frame)
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
}
