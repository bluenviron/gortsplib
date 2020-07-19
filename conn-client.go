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
	clientReadBufferSize  = 4096
	clientWriteBufferSize = 4096
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
	auth    *authClient
}

// NewConnClient allocates a ConnClient. See ConnClientConf for the options.
func NewConnClient(conf ConnClientConf) *ConnClient {
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
	}
}

// NetConn returns the underlying net.Conn.
func (c *ConnClient) NetConn() net.Conn {
	return c.conf.Conn
}

// ReadFrame reads an InterleavedFrame.
func (c *ConnClient) ReadFrame(frame *InterleavedFrame) error {
	c.conf.Conn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
	return frame.Read(c.br)
}

// ReadFrameOrResponse reads an InterleavedFrame or a Response.
func (c *ConnClient) ReadFrameOrResponse(frame *InterleavedFrame) (interface{}, error) {
	c.conf.Conn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
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
	c.curCSeq += 1
	req.Header["CSeq"] = HeaderValue{strconv.FormatInt(int64(c.curCSeq), 10)}

	c.conf.Conn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	err := req.Write(c.bw)
	if err != nil {
		return nil, err
	}

	if req.SkipResponse {
		return nil, nil
	}

	c.conf.Conn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))
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
	c.conf.Conn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
	return frame.Write(c.bw)
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
	rtcpPort int) (int, int, *Response, error) {

	res, err := c.setup(u, track.Media, []string{
		"RTP/AVP/UDP",
		"unicast",
		fmt.Sprintf("client_port=%d-%d", rtpPort, rtcpPort),
	})
	if err != nil {
		return 0, 0, nil, err
	}

	th, err := ReadHeaderTransport(res.Header["Transport"])
	if err != nil {
		return 0, 0, nil, fmt.Errorf("SETUP: transport header: %s", err)
	}

	rtpServerPort, rtcpServerPort := th.Ports("server_port")
	if rtpServerPort == 0 {
		return 0, 0, nil, fmt.Errorf("SETUP: server ports not provided")
	}

	return rtpServerPort, rtcpServerPort, res, nil
}

// SetupTcp writes a SETUP request, that means that we want to read
// a given track with the TCP transport. It then reads a Response.
func (c *ConnClient) SetupTcp(u *url.URL, track *Track) (*Response, error) {
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
		return nil, fmt.Errorf("SETUP: transport header does not have %s (%s)", interleaved, res.Header["Transport"])
	}

	return res, nil
}

// Play writes a PLAY request, that means that we want to start the
// stream. It then reads a Response.
func (c *ConnClient) Play(u *url.URL) (*Response, error) {
	_, err := c.Do(&Request{
		Method:       PLAY,
		Url:          u,
		SkipResponse: true,
	})
	if err != nil {
		return nil, err
	}

	frame := &InterleavedFrame{
		Content: make([]byte, 0, 512*1024),
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
}
