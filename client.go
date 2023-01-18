/*
Package gortsplib is a RTSP 1.0 library for the Go programming language,
written for rtsp-simple-server.

Examples are available at https://github.com/aler9/gortsplib/tree/master/examples
*/
package gortsplib

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/auth"
	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/bytecounter"
	"github.com/aler9/gortsplib/v2/pkg/conn"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/headers"
	"github.com/aler9/gortsplib/v2/pkg/liberrors"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/aler9/gortsplib/v2/pkg/sdp"
	"github.com/aler9/gortsplib/v2/pkg/url"
)

func isAnyPort(p int) bool {
	return p == 0 || p == 1
}

func findBaseURL(sd *sdp.SessionDescription, res *base.Response, u *url.URL) (*url.URL, error) {
	// use global control attribute
	if control, ok := sd.Attribute("control"); ok && control != "*" {
		ret, err := url.Parse(control)
		if err != nil {
			return nil, fmt.Errorf("invalid control attribute: '%v'", control)
		}

		// add credentials
		ret.User = u.User

		return ret, nil
	}

	// use Content-Base
	if cb, ok := res.Header["Content-Base"]; ok {
		if len(cb) != 1 {
			return nil, fmt.Errorf("invalid Content-Base: '%v'", cb)
		}

		ret, err := url.Parse(cb[0])
		if err != nil {
			return nil, fmt.Errorf("invalid Content-Base: '%v'", cb)
		}

		// add credentials
		ret.User = u.User

		return ret, nil
	}

	// use URL of request
	return u, nil
}

type clientState int

const (
	clientStateInitial clientState = iota
	clientStatePrePlay
	clientStatePlay
	clientStatePreRecord
	clientStateRecord
)

func (s clientState) String() string {
	switch s {
	case clientStateInitial:
		return "initial"
	case clientStatePrePlay:
		return "prePlay"
	case clientStatePlay:
		return "play"
	case clientStatePreRecord:
		return "preRecord"
	case clientStateRecord:
		return "record"
	}
	return "unknown"
}

type optionsReq struct {
	url *url.URL
	res chan clientRes
}

type describeReq struct {
	url *url.URL
	res chan clientRes
}

type announceReq struct {
	url    *url.URL
	medias media.Medias
	res    chan clientRes
}

type setupReq struct {
	media    *media.Media
	baseURL  *url.URL
	rtpPort  int
	rtcpPort int
	res      chan clientRes
}

type playReq struct {
	ra  *headers.Range
	res chan clientRes
}

type recordReq struct {
	res chan clientRes
}

type pauseReq struct {
	res chan clientRes
}

type clientRes struct {
	medias  media.Medias
	baseURL *url.URL
	res     *base.Response
	err     error
}

// Client is a RTSP client.
type Client struct {
	//
	// RTSP parameters (all optional)
	//
	// timeout of read operations.
	// It defaults to 10 seconds.
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds.
	WriteTimeout time.Duration
	// a TLS configuration to connect to TLS (RTSPS) servers.
	// It defaults to nil.
	TLSConfig *tls.Config
	// disable being redirected to other servers, that can happen during Describe().
	// It defaults to false.
	RedirectDisable bool
	// enable communication with servers which don't provide server ports or use
	// different server ports than the ones announced.
	// This can be a security issue.
	// It defaults to false.
	AnyPortEnable bool
	// the stream transport (UDP, Multicast or TCP).
	// If nil, it is chosen automatically (first UDP, then, if it fails, TCP).
	// It defaults to nil.
	Transport *Transport
	// If the client is reading with UDP, it must receive
	// at least a packet within this timeout.
	// It defaults to 3 seconds.
	InitialUDPReadTimeout time.Duration
	// read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It defaults to 256.
	ReadBufferCount int
	// write buffer count.
	// It allows to queue packets before sending them.
	// It defaults to 256.
	WriteBufferCount int
	// user agent header
	// It defaults to "gortsplib"
	UserAgent string
	// disable automatic RTCP sender reports.
	DisableRTCPSenderReports bool
	// pointer to a variable that stores received bytes.
	BytesReceived *uint64
	// pointer to a variable that stores sent bytes.
	BytesSent *uint64

	//
	// system functions (all optional)
	//
	// function used to initialize the TCP client.
	// It defaults to (&net.Dialer{}).DialContext.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
	// function used to initialize UDP listeners.
	// It defaults to net.ListenPacket.
	ListenPacket func(network, address string) (net.PacketConn, error)

	//
	// callbacks (all optional)
	//
	// called before every request.
	OnRequest func(*base.Request)
	// called after every response.
	OnResponse func(*base.Response)
	// called when there's a non-fatal decoding error of RTP or RTCP packets.
	OnDecodeError func(error)

	//
	// private
	//

	senderReportPeriod      time.Duration
	udpReceiverReportPeriod time.Duration
	checkStreamPeriod       time.Duration
	keepalivePeriod         time.Duration

	scheme             string
	host               string
	ctx                context.Context
	ctxCancel          func()
	state              clientState
	nconn              net.Conn
	conn               *conn.Conn
	session            string
	sender             *auth.Sender
	cseq               int
	optionsSent        bool
	useGetParameter    bool
	lastDescribeURL    *url.URL
	baseURL            *url.URL
	effectiveTransport *Transport
	medias             map[*media.Media]*clientMedia
	tcpMediasByChannel map[int]*clientMedia
	lastRange          *headers.Range
	checkStreamTimer   *time.Timer
	checkStreamInitial bool
	tcpLastFrameTime   *int64
	keepaliveTimer     *time.Timer
	closeError         error
	writer             writer

	// connCloser channels
	connCloserTerminate chan struct{}
	connCloserDone      chan struct{}

	// reader channels
	readerErr chan error

	// in
	options  chan optionsReq
	describe chan describeReq
	announce chan announceReq
	setup    chan setupReq
	play     chan playReq
	record   chan recordReq
	pause    chan pauseReq

	// out
	done chan struct{}
}

// Start initializes the connection to a server.
func (c *Client) Start(scheme string, host string) error {
	// RTSP parameters
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 10 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.InitialUDPReadTimeout == 0 {
		c.InitialUDPReadTimeout = 3 * time.Second
	}
	if c.ReadBufferCount == 0 {
		c.ReadBufferCount = 256
	}
	if c.WriteBufferCount == 0 {
		c.WriteBufferCount = 256
	}
	if (c.WriteBufferCount & (c.WriteBufferCount - 1)) != 0 {
		return fmt.Errorf("WriteBufferCount must be a power of two")
	}
	if c.UserAgent == "" {
		c.UserAgent = "gortsplib"
	}
	if c.BytesReceived == nil {
		c.BytesReceived = new(uint64)
	}
	if c.BytesSent == nil {
		c.BytesSent = new(uint64)
	}

	// system functions
	if c.DialContext == nil {
		c.DialContext = (&net.Dialer{}).DialContext
	}
	if c.ListenPacket == nil {
		c.ListenPacket = net.ListenPacket
	}

	// callbacks
	if c.OnRequest == nil {
		c.OnRequest = func(*base.Request) {
		}
	}
	if c.OnResponse == nil {
		c.OnResponse = func(*base.Response) {
		}
	}
	if c.OnDecodeError == nil {
		c.OnDecodeError = func(error) {
		}
	}

	// private
	if c.senderReportPeriod == 0 {
		c.senderReportPeriod = 10 * time.Second
	}
	if c.udpReceiverReportPeriod == 0 {
		c.udpReceiverReportPeriod = 10 * time.Second
	}
	if c.checkStreamPeriod == 0 {
		c.checkStreamPeriod = 1 * time.Second
	}
	if c.keepalivePeriod == 0 {
		c.keepalivePeriod = 30 * time.Second
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	c.scheme = scheme
	c.host = host
	c.ctx = ctx
	c.ctxCancel = ctxCancel
	c.checkStreamTimer = emptyTimer()
	c.keepaliveTimer = emptyTimer()
	c.options = make(chan optionsReq)
	c.describe = make(chan describeReq)
	c.announce = make(chan announceReq)
	c.setup = make(chan setupReq)
	c.play = make(chan playReq)
	c.record = make(chan recordReq)
	c.pause = make(chan pauseReq)
	c.done = make(chan struct{})

	go c.run()

	return nil
}

// StartRecording connects to the address and starts publishing given media.
func (c *Client) StartRecording(address string, medias media.Medias) error {
	u, err := url.Parse(address)
	if err != nil {
		return err
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	_, err = c.Announce(u, medias)
	if err != nil {
		c.Close()
		return err
	}

	err = c.SetupAll(medias, u)
	if err != nil {
		c.Close()
		return err
	}

	_, err = c.Record()
	if err != nil {
		c.Close()
		return err
	}

	return nil
}

// Close closes all client resources and waits for them to close.
func (c *Client) Close() error {
	c.ctxCancel()
	<-c.done
	return c.closeError
}

// Wait waits until all client resources are closed.
// This can happen when a fatal error occurs or when Close() is called.
func (c *Client) Wait() error {
	<-c.done
	return c.closeError
}

func (c *Client) run() {
	defer close(c.done)

	c.closeError = c.runInner()

	c.ctxCancel()

	c.doClose()
}

func (c *Client) runInner() error {
	for {
		select {
		case req := <-c.options:
			res, err := c.doOptions(req.url)
			req.res <- clientRes{res: res, err: err}

		case req := <-c.describe:
			medias, baseURL, res, err := c.doDescribe(req.url)
			req.res <- clientRes{medias: medias, baseURL: baseURL, res: res, err: err}

		case req := <-c.announce:
			res, err := c.doAnnounce(req.url, req.medias)
			req.res <- clientRes{res: res, err: err}

		case req := <-c.setup:
			res, err := c.doSetup(req.media, req.baseURL, req.rtpPort, req.rtcpPort)
			req.res <- clientRes{res: res, err: err}

		case req := <-c.play:
			res, err := c.doPlay(req.ra, false)
			req.res <- clientRes{res: res, err: err}

		case req := <-c.record:
			res, err := c.doRecord()
			req.res <- clientRes{res: res, err: err}

		case req := <-c.pause:
			res, err := c.doPause()
			req.res <- clientRes{res: res, err: err}

		case <-c.checkStreamTimer.C:
			if *c.effectiveTransport == TransportUDP ||
				*c.effectiveTransport == TransportUDPMulticast {
				if c.checkStreamInitial {
					c.checkStreamInitial = false

					// check that at least one packet has been received
					inTimeout := func() bool {
						for _, ct := range c.medias {
							lft := atomic.LoadInt64(ct.udpRTPListener.lastPacketTime)
							if lft != 0 {
								return false
							}

							lft = atomic.LoadInt64(ct.udpRTCPListener.lastPacketTime)
							if lft != 0 {
								return false
							}
						}
						return true
					}()
					if inTimeout {
						err := c.trySwitchingProtocol()
						if err != nil {
							return err
						}
					}
				} else {
					inTimeout := func() bool {
						now := time.Now()
						for _, ct := range c.medias {
							lft := time.Unix(atomic.LoadInt64(ct.udpRTPListener.lastPacketTime), 0)
							if now.Sub(lft) < c.ReadTimeout {
								return false
							}

							lft = time.Unix(atomic.LoadInt64(ct.udpRTCPListener.lastPacketTime), 0)
							if now.Sub(lft) < c.ReadTimeout {
								return false
							}
						}
						return true
					}()
					if inTimeout {
						return liberrors.ErrClientUDPTimeout{}
					}
				}
			} else { // TCP
				inTimeout := func() bool {
					now := time.Now()
					lft := time.Unix(atomic.LoadInt64(c.tcpLastFrameTime), 0)
					return now.Sub(lft) >= c.ReadTimeout
				}()
				if inTimeout {
					return liberrors.ErrClientTCPTimeout{}
				}
			}

			c.checkStreamTimer = time.NewTimer(c.checkStreamPeriod)

		case <-c.keepaliveTimer.C:
			_, err := c.do(&base.Request{
				Method: func() base.Method {
					// the VLC integrated rtsp server requires GET_PARAMETER
					if c.useGetParameter {
						return base.GetParameter
					}
					return base.Options
				}(),
				// use the stream base URL, otherwise some cameras do not reply
				URL: c.baseURL,
			}, true, false)
			if err != nil {
				return err
			}

			c.keepaliveTimer = time.NewTimer(c.keepalivePeriod)

		case err := <-c.readerErr:
			c.readerErr = nil
			return err

		case <-c.ctx.Done():
			return liberrors.ErrClientTerminated{}
		}
	}
}

func (c *Client) doClose() {
	if c.state == clientStatePlay || c.state == clientStateRecord {
		c.playRecordStop(true)

		c.do(&base.Request{
			Method: base.Teardown,
			URL:    c.baseURL,
		}, true, false)

		c.nconn.Close()
		c.nconn = nil
		c.conn = nil
	} else if c.nconn != nil {
		c.connCloserStop()
		c.nconn.Close()
		c.nconn = nil
		c.conn = nil
	}

	for _, cm := range c.medias {
		cm.close()
	}
}

func (c *Client) reset() {
	c.doClose()

	c.state = clientStateInitial
	c.session = ""
	c.sender = nil
	c.cseq = 0
	c.optionsSent = false
	c.useGetParameter = false
	c.baseURL = nil
	c.effectiveTransport = nil
	c.medias = nil
	c.tcpMediasByChannel = nil
}

func (c *Client) checkState(allowed map[clientState]struct{}) error {
	if _, ok := allowed[c.state]; ok {
		return nil
	}

	allowedList := make([]fmt.Stringer, len(allowed))
	i := 0
	for a := range allowed {
		allowedList[i] = a
		i++
	}

	return liberrors.ErrClientInvalidState{AllowedList: allowedList, State: c.state}
}

func (c *Client) trySwitchingProtocol() error {
	prevScheme := c.scheme
	prevHost := c.host
	prevBaseURL := c.baseURL
	prevMedias := c.medias

	c.reset()

	v := TransportTCP
	c.effectiveTransport = &v
	c.scheme = prevScheme
	c.host = prevHost

	// some Hikvision cameras require a describe before a setup
	_, _, _, err := c.doDescribe(c.lastDescribeURL)
	if err != nil {
		return err
	}

	for i, cm := range prevMedias {
		_, err := c.doSetup(cm.media, prevBaseURL, 0, 0)
		if err != nil {
			return err
		}

		c.medias[i].onPacketRTCP = cm.onPacketRTCP
		for j, tr := range cm.formats {
			c.medias[i].formats[j].onPacketRTP = tr.onPacketRTP
		}
	}

	_, err = c.doPlay(c.lastRange, true)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) trySwitchingProtocol2(medi *media.Media, baseURL *url.URL) (*base.Response, error) {
	prevScheme := c.scheme
	prevHost := c.host

	c.reset()

	v := TransportTCP
	c.effectiveTransport = &v
	c.scheme = prevScheme
	c.host = prevHost

	// some Hikvision cameras require a describe before a setup
	_, _, _, err := c.doDescribe(c.lastDescribeURL)
	if err != nil {
		return nil, err
	}

	return c.doSetup(medi, baseURL, 0, 0)
}

func (c *Client) playRecordStart() {
	// stop connCloser
	c.connCloserStop()

	if c.state == clientStatePlay {
		c.keepaliveTimer = time.NewTimer(c.keepalivePeriod)

		switch *c.effectiveTransport {
		case TransportUDP:
			c.checkStreamTimer = time.NewTimer(c.InitialUDPReadTimeout)
			c.checkStreamInitial = true

		case TransportUDPMulticast:
			c.checkStreamTimer = time.NewTimer(c.checkStreamPeriod)

		default: // TCP
			c.checkStreamTimer = time.NewTimer(c.checkStreamPeriod)
			v := time.Now().Unix()
			c.tcpLastFrameTime = &v
		}
	}

	if c.state == clientStatePlay {
		// when reading, buffer is only used to send RTCP receiver reports,
		// that are much smaller than RTP packets and are sent at a fixed interval.
		// decrease RAM consumption by allocating less buffers.
		c.writer.allocateBuffer(8)
	} else {
		c.writer.allocateBuffer(c.WriteBufferCount)
	}

	c.writer.start()

	for _, cm := range c.medias {
		cm.start()
	}

	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we disable the deadline and perform a check with a ticker.
	c.nconn.SetReadDeadline(time.Time{})

	// start reader
	c.readerErr = make(chan error)
	go c.runReader()
}

func (c *Client) runReader() {
	c.readerErr <- func() error {
		if *c.effectiveTransport == TransportUDP || *c.effectiveTransport == TransportUDPMulticast {
			for {
				_, err := c.conn.ReadResponse()
				if err != nil {
					return err
				}
			}
		} else {
			for {
				what, err := c.conn.ReadInterleavedFrameOrResponse()
				if err != nil {
					return err
				}

				if fr, ok := what.(*base.InterleavedFrame); ok {
					channel := fr.Channel
					isRTP := true
					if (channel % 2) != 0 {
						channel--
						isRTP = false
					}

					media, ok := c.tcpMediasByChannel[channel]
					if !ok {
						continue
					}

					if isRTP {
						err = media.readRTP(fr.Payload)
					} else {
						err = media.readRTCP(fr.Payload)
					}
					if err != nil {
						return err
					}
				}
			}
		}
	}()
}

func (c *Client) playRecordStop(isClosing bool) {
	// stop reader
	if c.readerErr != nil {
		c.nconn.SetReadDeadline(time.Now())
		<-c.readerErr
	}

	// stop timers
	c.checkStreamTimer = emptyTimer()
	c.keepaliveTimer = emptyTimer()

	c.writer.stop()

	for _, cm := range c.medias {
		cm.stop()
	}

	// start connCloser
	if !isClosing {
		c.connCloserStart()
	}
}

func (c *Client) connOpen() error {
	if c.scheme != "rtsp" && c.scheme != "rtsps" {
		return fmt.Errorf("unsupported scheme '%s'", c.scheme)
	}

	if c.scheme == "rtsps" && c.Transport != nil && *c.Transport != TransportTCP {
		return fmt.Errorf("RTSPS can be used only with TCP")
	}

	// add default port
	_, _, err := net.SplitHostPort(c.host)
	if err != nil {
		if c.scheme == "rtsp" {
			c.host = net.JoinHostPort(c.host, "554")
		} else { // rtsps
			c.host = net.JoinHostPort(c.host, "322")
		}
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.ReadTimeout)
	defer cancel()

	nconn, err := c.DialContext(ctx, "tcp", c.host)
	if err != nil {
		return err
	}

	if c.scheme == "rtsps" {
		tlsConfig := c.TLSConfig

		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}

		host, _, _ := net.SplitHostPort(c.host)
		tlsConfig.ServerName = host

		nconn = tls.Client(nconn, tlsConfig)
	}

	c.nconn = nconn
	bc := bytecounter.New(c.nconn, c.BytesReceived, c.BytesSent)
	c.conn = conn.NewConn(bc)

	c.connCloserStart()
	return nil
}

func (c *Client) connCloserStart() {
	c.connCloserTerminate = make(chan struct{})
	c.connCloserDone = make(chan struct{})

	go func() {
		defer close(c.connCloserDone)

		select {
		case <-c.ctx.Done():
			c.nconn.Close()

		case <-c.connCloserTerminate:
		}
	}()
}

func (c *Client) connCloserStop() {
	close(c.connCloserTerminate)
	<-c.connCloserDone
	c.connCloserDone = nil
}

func (c *Client) do(req *base.Request, skipResponse bool, allowFrames bool) (*base.Response, error) {
	if c.nconn == nil {
		err := c.connOpen()
		if err != nil {
			return nil, err
		}
	}

	if !c.optionsSent && req.Method != base.Options {
		_, err := c.doOptions(req.URL)
		if err != nil {
			return nil, err
		}
	}

	if req.Header == nil {
		req.Header = make(base.Header)
	}

	if c.session != "" {
		req.Header["Session"] = base.HeaderValue{c.session}
	}

	c.cseq++
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(c.cseq), 10)}

	req.Header["User-Agent"] = base.HeaderValue{c.UserAgent}

	if c.sender != nil {
		c.sender.AddAuthorization(req)
	}

	c.OnRequest(req)

	c.nconn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	err := c.conn.WriteRequest(req)
	if err != nil {
		return nil, err
	}

	if skipResponse {
		return nil, nil
	}

	c.nconn.SetReadDeadline(time.Now().Add(c.ReadTimeout))
	var res *base.Response
	if allowFrames {
		// read the response and ignore interleaved frames in between;
		// interleaved frames are sent in two cases:
		// * when the server is v4lrtspserver, before the PLAY response
		// * when the stream is already playing
		res, err = c.conn.ReadResponseIgnoreFrames()
	} else {
		res, err = c.conn.ReadResponse()
	}
	if err != nil {
		return nil, err
	}

	c.OnResponse(res)

	// get session from response
	if v, ok := res.Header["Session"]; ok {
		var sx headers.Session
		err := sx.Unmarshal(v)
		if err != nil {
			return nil, liberrors.ErrClientSessionHeaderInvalid{Err: err}
		}
		c.session = sx.Session

		if sx.Timeout != nil && *sx.Timeout > 0 {
			c.keepalivePeriod = time.Duration(float64(*sx.Timeout)*0.8) * time.Second
		}
	}

	// if required, send request again with authentication
	if res.StatusCode == base.StatusUnauthorized && req.URL.User != nil && c.sender == nil {
		pass, _ := req.URL.User.Password()
		user := req.URL.User.Username()

		sender, err := auth.NewSender(res.Header["WWW-Authenticate"], user, pass)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		c.sender = sender

		return c.do(req, skipResponse, allowFrames)
	}

	return res, nil
}

func (c *Client) doOptions(u *url.URL) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial:   {},
		clientStatePrePlay:   {},
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.do(&base.Request{
		Method: base.Options,
		URL:    u,
	}, false, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		// since this method is not implemented by every RTSP server,
		// return only if status code is not 404
		if res.StatusCode == base.StatusNotFound {
			return res, nil
		}
		return nil, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	c.optionsSent = true

	c.useGetParameter = func() bool {
		pub, ok := res.Header["Public"]
		if !ok || len(pub) != 1 {
			return false
		}

		for _, m := range strings.Split(pub[0], ",") {
			if base.Method(strings.Trim(m, " ")) == base.GetParameter {
				return true
			}
		}
		return false
	}()

	return res, nil
}

// Options writes an OPTIONS request and reads a response.
func (c *Client) Options(u *url.URL) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.options <- optionsReq{url: u, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doDescribe(u *url.URL) (media.Medias, *url.URL, *base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial:   {},
		clientStatePrePlay:   {},
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, nil, nil, err
	}

	res, err := c.do(&base.Request{
		Method: base.Describe,
		URL:    u,
		Header: base.Header{
			"Accept": base.HeaderValue{"application/sdp"},
		},
	}, false, false)
	if err != nil {
		return nil, nil, nil, err
	}

	if res.StatusCode != base.StatusOK {
		// redirect
		if !c.RedirectDisable &&
			res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {
			c.reset()

			ru, err := url.Parse(res.Header["Location"][0])
			if err != nil {
				return nil, nil, nil, err
			}

			if u.User != nil {
				ru.User = u.User
			}

			c.scheme = ru.Scheme
			c.host = ru.Host

			return c.doDescribe(ru)
		}

		return nil, nil, res, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	ct, ok := res.Header["Content-Type"]
	if !ok || len(ct) != 1 {
		return nil, nil, nil, liberrors.ErrClientContentTypeMissing{}
	}

	// strip encoding information from Content-Type header
	ct = base.HeaderValue{strings.Split(ct[0], ";")[0]}

	if ct[0] != "application/sdp" {
		return nil, nil, nil, liberrors.ErrClientContentTypeUnsupported{CT: ct}
	}

	var sd sdp.SessionDescription
	err = sd.Unmarshal(res.Body)
	if err != nil {
		return nil, nil, nil, err
	}

	var medias media.Medias
	err = medias.Unmarshal(sd.MediaDescriptions)
	if err != nil {
		return nil, nil, nil, err
	}

	baseURL, err := findBaseURL(&sd, res, u)
	if err != nil {
		return nil, nil, nil, err
	}

	c.lastDescribeURL = u

	return medias, baseURL, res, nil
}

// Describe writes a DESCRIBE request and reads a Response.
func (c *Client) Describe(u *url.URL) (media.Medias, *url.URL, *base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.describe <- describeReq{url: u, res: cres}:
		res := <-cres
		return res.medias, res.baseURL, res.res, res.err

	case <-c.ctx.Done():
		return nil, nil, nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doAnnounce(u *url.URL, medias media.Medias) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial: {},
	})
	if err != nil {
		return nil, err
	}

	medias.SetControls()

	byts, err := medias.Marshal(false).Marshal()
	if err != nil {
		return nil, err
	}

	res, err := c.do(&base.Request{
		Method: base.Announce,
		URL:    u,
		Header: base.Header{
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: byts,
	}, false, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.baseURL = u.Clone()
	c.state = clientStatePreRecord

	return res, nil
}

// Announce writes an ANNOUNCE request and reads a Response.
func (c *Client) Announce(u *url.URL, medias media.Medias) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.announce <- announceReq{url: u, medias: medias, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doSetup(
	medi *media.Media,
	baseURL *url.URL,
	rtpPort int,
	rtcpPort int,
) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial:   {},
		clientStatePrePlay:   {},
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	if c.baseURL != nil && *baseURL != *c.baseURL {
		return nil, liberrors.ErrClientCannotSetupMediasDifferentURLs{}
	}

	// always use TCP if encrypted
	if c.scheme == "rtsps" {
		v := TransportTCP
		c.effectiveTransport = &v
	}

	requestedTransport := func() Transport {
		// transport set by previous Setup() or trySwitchingProtocol()
		if c.effectiveTransport != nil {
			return *c.effectiveTransport
		}

		// transport set by conf
		if c.Transport != nil {
			return *c.Transport
		}

		// try UDP
		return TransportUDP
	}()

	mode := headers.TransportModePlay
	if c.state == clientStatePreRecord {
		mode = headers.TransportModeRecord
	}

	th := headers.Transport{
		Mode: &mode,
	}

	cm := newClientMedia(c)

	switch requestedTransport {
	case TransportUDP:
		if (rtpPort == 0 && rtcpPort != 0) ||
			(rtpPort != 0 && rtcpPort == 0) {
			return nil, liberrors.ErrClientUDPPortsZero{}
		}

		if rtpPort != 0 && rtcpPort != (rtpPort+1) {
			return nil, liberrors.ErrClientUDPPortsNotConsecutive{}
		}

		err := cm.allocateUDPListeners(
			false,
			":"+strconv.FormatInt(int64(rtpPort), 10),
			":"+strconv.FormatInt(int64(rtcpPort), 10),
		)
		if err != nil {
			return nil, err
		}

		v1 := headers.TransportDeliveryUnicast
		th.Delivery = &v1
		th.Protocol = headers.TransportProtocolUDP
		th.ClientPorts = &[2]int{cm.udpRTPListener.port(), cm.udpRTCPListener.port()}

	case TransportUDPMulticast:
		v1 := headers.TransportDeliveryMulticast
		th.Delivery = &v1
		th.Protocol = headers.TransportProtocolUDP

	case TransportTCP:
		v1 := headers.TransportDeliveryUnicast
		th.Delivery = &v1
		th.Protocol = headers.TransportProtocolTCP
		mediaCount := len(c.medias)
		th.InterleavedIDs = &[2]int{(mediaCount * 2), (mediaCount * 2) + 1}
	}

	mediaURL, err := medi.URL(baseURL)
	if err != nil {
		cm.close()
		return nil, err
	}

	res, err := c.do(&base.Request{
		Method: base.Setup,
		URL:    mediaURL,
		Header: base.Header{
			"Transport": th.Marshal(),
		},
	}, false, false)
	if err != nil {
		cm.close()
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		cm.close()

		// switch transport automatically
		if res.StatusCode == base.StatusUnsupportedTransport &&
			c.effectiveTransport == nil &&
			c.Transport == nil {
			v := TransportTCP
			c.effectiveTransport = &v

			return c.doSetup(medi, baseURL, 0, 0)
		}

		return nil, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	var thRes headers.Transport
	err = thRes.Unmarshal(res.Header["Transport"])
	if err != nil {
		cm.close()
		return nil, liberrors.ErrClientTransportHeaderInvalid{Err: err}
	}

	switch requestedTransport {
	case TransportUDP, TransportUDPMulticast:
		if thRes.Protocol == headers.TransportProtocolTCP {
			cm.close()

			// switch transport automatically
			if c.effectiveTransport == nil &&
				c.Transport == nil {
				return c.trySwitchingProtocol2(medi, baseURL)
			}

			return nil, liberrors.ErrClientServerRequestedTCP{}
		}
	}

	switch requestedTransport {
	case TransportUDP:
		if thRes.Delivery != nil && *thRes.Delivery != headers.TransportDeliveryUnicast {
			cm.close()
			return nil, liberrors.ErrClientTransportHeaderInvalidDelivery{}
		}

		if c.state == clientStatePreRecord || !c.AnyPortEnable {
			if thRes.ServerPorts == nil || isAnyPort(thRes.ServerPorts[0]) || isAnyPort(thRes.ServerPorts[1]) {
				cm.close()
				return nil, liberrors.ErrClientServerPortsNotProvided{}
			}
		}

		if thRes.Source != nil {
			cm.udpRTPListener.readIP = *thRes.Source
		} else {
			cm.udpRTPListener.readIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		}

		if thRes.ServerPorts != nil {
			if !c.AnyPortEnable {
				cm.udpRTPListener.readPort = thRes.ServerPorts[0]
			}
			cm.udpRTPListener.writeAddr = &net.UDPAddr{
				IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
				Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
				Port: thRes.ServerPorts[0],
			}
		}

		if thRes.Source != nil {
			cm.udpRTCPListener.readIP = *thRes.Source
		} else {
			cm.udpRTCPListener.readIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		}

		if thRes.ServerPorts != nil {
			if !c.AnyPortEnable {
				cm.udpRTCPListener.readPort = thRes.ServerPorts[1]
			}
			cm.udpRTCPListener.writeAddr = &net.UDPAddr{
				IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
				Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
				Port: thRes.ServerPorts[1],
			}
		}

	case TransportUDPMulticast:
		if thRes.Delivery == nil || *thRes.Delivery != headers.TransportDeliveryMulticast {
			return nil, liberrors.ErrClientTransportHeaderInvalidDelivery{}
		}

		if thRes.Ports == nil {
			return nil, liberrors.ErrClientTransportHeaderNoPorts{}
		}

		if thRes.Destination == nil {
			return nil, liberrors.ErrClientTransportHeaderNoDestination{}
		}

		err := cm.allocateUDPListeners(
			true,
			thRes.Destination.String()+":"+strconv.FormatInt(int64(thRes.Ports[0]), 10),
			thRes.Destination.String()+":"+strconv.FormatInt(int64(thRes.Ports[1]), 10),
		)
		if err != nil {
			return nil, err
		}

		cm.udpRTPListener.readIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		cm.udpRTPListener.readPort = thRes.Ports[0]
		cm.udpRTPListener.writeAddr = &net.UDPAddr{
			IP:   *thRes.Destination,
			Port: thRes.Ports[0],
		}

		cm.udpRTCPListener.readIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		cm.udpRTCPListener.readPort = thRes.Ports[1]
		cm.udpRTCPListener.writeAddr = &net.UDPAddr{
			IP:   *thRes.Destination,
			Port: thRes.Ports[1],
		}

	case TransportTCP:
		if thRes.Protocol != headers.TransportProtocolTCP {
			return nil, liberrors.ErrClientServerRequestedUDP{}
		}

		if thRes.Delivery != nil && *thRes.Delivery != headers.TransportDeliveryUnicast {
			return nil, liberrors.ErrClientTransportHeaderInvalidDelivery{}
		}

		if thRes.InterleavedIDs == nil {
			return nil, liberrors.ErrClientTransportHeaderNoInterleavedIDs{}
		}

		if (thRes.InterleavedIDs[0]%2) != 0 ||
			(thRes.InterleavedIDs[0]+1) != thRes.InterleavedIDs[1] {
			return nil, liberrors.ErrClientTransportHeaderInvalidInterleavedIDs{}
		}

		if _, ok := c.tcpMediasByChannel[thRes.InterleavedIDs[0]]; ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrClientTransportHeaderInterleavedIDsAlreadyUsed{}
		}

		if c.tcpMediasByChannel == nil {
			c.tcpMediasByChannel = make(map[int]*clientMedia)
		}

		c.tcpMediasByChannel[thRes.InterleavedIDs[0]] = cm
		cm.tcpChannel = thRes.InterleavedIDs[0]
	}

	if c.medias == nil {
		c.medias = make(map[*media.Media]*clientMedia)
	}

	c.medias[medi] = cm
	cm.setMedia(medi)

	c.baseURL = baseURL
	c.effectiveTransport = &requestedTransport

	if mode == headers.TransportModePlay {
		c.state = clientStatePrePlay
	} else {
		c.state = clientStatePreRecord
	}

	return res, nil
}

// Setup writes a SETUP request and reads a Response.
// rtpPort and rtcpPort are used only if transport is UDP.
// if rtpPort and rtcpPort are zero, they are chosen automatically.
func (c *Client) Setup(
	media *media.Media,
	baseURL *url.URL,
	rtpPort int,
	rtcpPort int,
) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.setup <- setupReq{
		media:    media,
		baseURL:  baseURL,
		rtpPort:  rtpPort,
		rtcpPort: rtcpPort,
		res:      cres,
	}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

// SetupAll setups all the given medias.
func (c *Client) SetupAll(medias media.Medias, baseURL *url.URL) error {
	for _, m := range medias {
		_, err := c.Setup(m, baseURL, 0, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) doPlay(ra *headers.Range, isSwitchingProtocol bool) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	// open the firewall by sending test packets to the counterpart.
	// do this before sending the request.
	// don't do this with multicast, otherwise the RTP packet is going to be broadcasted
	// to all listeners, including us, messing up the stream.
	if *c.effectiveTransport == TransportUDP {
		for _, ct := range c.medias {
			byts, _ := (&rtp.Packet{Header: rtp.Header{Version: 2}}).Marshal()
			ct.udpRTPListener.write(byts)

			byts, _ = (&rtcp.ReceiverReport{}).Marshal()
			ct.udpRTCPListener.write(byts)
		}
	}

	// Range is mandatory in Parrot Streaming Server
	if ra == nil {
		ra = &headers.Range{
			Value: &headers.RangeNPT{
				Start: 0,
			},
		}
	}

	res, err := c.do(&base.Request{
		Method: base.Play,
		URL:    c.baseURL,
		Header: base.Header{
			"Range": ra.Marshal(),
		},
	}, false, *c.effectiveTransport == TransportTCP)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.lastRange = ra
	c.state = clientStatePlay
	c.playRecordStart()

	return res, nil
}

// Play writes a PLAY request and reads a Response.
// This can be called only after Setup().
func (c *Client) Play(ra *headers.Range) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.play <- playReq{ra: ra, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doRecord() (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.do(&base.Request{
		Method: base.Record,
		URL:    c.baseURL,
	}, false, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.state = clientStateRecord
	c.playRecordStart()

	return nil, nil
}

// Record writes a RECORD request and reads a Response.
// This can be called only after Announce() and Setup().
func (c *Client) Record() (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.record <- recordReq{res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doPause() (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePlay:   {},
		clientStateRecord: {},
	})
	if err != nil {
		return nil, err
	}

	c.playRecordStop(false)

	// change state regardless of the response
	switch c.state {
	case clientStatePlay:
		c.state = clientStatePrePlay
	case clientStateRecord:
		c.state = clientStatePreRecord
	}

	res, err := c.do(&base.Request{
		Method: base.Pause,
		URL:    c.baseURL,
	}, false, *c.effectiveTransport == TransportTCP)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	return res, nil
}

// Pause writes a PAUSE request and reads a Response.
// This can be called only after Play() or Record().
func (c *Client) Pause() (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.pause <- pauseReq{res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

// Seek asks the server to re-start the stream from a specific timestamp.
func (c *Client) Seek(ra *headers.Range) (*base.Response, error) {
	_, err := c.Pause()
	if err != nil {
		return nil, err
	}

	return c.Play(ra)
}

// OnPacketRTPAny sets the callback that is called when a RTP packet is read from any setupped media.
func (c *Client) OnPacketRTPAny(cb func(*media.Media, format.Format, *rtp.Packet)) {
	for _, cm := range c.medias {
		cmedia := cm.media
		for _, forma := range cm.media.Formats {
			c.OnPacketRTP(cm.media, forma, func(pkt *rtp.Packet) {
				cb(cmedia, forma, pkt)
			})
		}
	}
}

// OnPacketRTCPAny sets the callback that is called when a RTCP packet is read from any setupped media.
func (c *Client) OnPacketRTCPAny(cb func(*media.Media, rtcp.Packet)) {
	for _, cm := range c.medias {
		cmedia := cm.media
		c.OnPacketRTCP(cm.media, func(pkt rtcp.Packet) {
			cb(cmedia, pkt)
		})
	}
}

// OnPacketRTP sets the callback that is called when a RTP packet is read.
func (c *Client) OnPacketRTP(medi *media.Media, forma format.Format, cb func(*rtp.Packet)) {
	cm := c.medias[medi]
	ct := cm.formats[forma.PayloadType()]
	ct.onPacketRTP = cb
}

// OnPacketRTCP sets the callback that is called when a RTCP packet is read.
func (c *Client) OnPacketRTCP(medi *media.Media, cb func(rtcp.Packet)) {
	cm := c.medias[medi]
	cm.onPacketRTCP = cb
}

// WritePacketRTP writes a RTP packet to the media stream.
func (c *Client) WritePacketRTP(medi *media.Media, pkt *rtp.Packet) error {
	return c.WritePacketRTPWithNTP(medi, pkt, time.Now())
}

// WritePacketRTPWithNTP writes a RTP packet to the media stream.
func (c *Client) WritePacketRTPWithNTP(medi *media.Media, pkt *rtp.Packet, ntp time.Time) error {
	cm := c.medias[medi]
	ct := cm.formats[pkt.PayloadType]
	return ct.writePacketRTPWithNTP(pkt, ntp)
}

// WritePacketRTCP writes a RTCP packet to the media stream.
func (c *Client) WritePacketRTCP(medi *media.Media, pkt rtcp.Packet) error {
	cm := c.medias[medi]
	return cm.writePacketRTCP(pkt)
}
