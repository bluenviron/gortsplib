/*
Package gortsplib is a RTSP library for the Go programming language.

Examples are available at https://github.com/bluenviron/gortsplib/tree/main/examples
*/
package gortsplib

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/internal/asyncprocessor"
	"github.com/bluenviron/gortsplib/v5/pkg/auth"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/bytecounter"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v5/pkg/mikey"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpreceiver"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpsender"
	"github.com/bluenviron/gortsplib/v5/pkg/rtptime"
	"github.com/bluenviron/gortsplib/v5/pkg/sdp"
)

const (
	clientUserAgent = "gortsplib"
)

func generateLocalSSRCs(existing []uint32, formats []format.Format) (map[uint8]uint32, error) {
	ret := make(map[uint8]uint32)

	for _, forma := range formats {
		for {
			ssrc, err := randUint32()
			if err != nil {
				return nil, err
			}

			if ssrc != 0 && !slices.Contains(existing, ssrc) {
				existing = append(existing, ssrc)
				ret[forma.PayloadType()] = ssrc
				break
			}
		}
	}

	return ret, nil
}

func ssrcsMapToList(m map[uint8]uint32) []uint32 {
	ret := make([]uint32, len(m))
	n := 0
	for _, el := range m {
		ret[n] = el
		n++
	}
	return ret
}

func clientExtractExistingSSRCs(setuppedMedias map[*description.Media]*clientMedia) []uint32 {
	var ret []uint32
	for _, media := range setuppedMedias {
		for _, forma := range media.formats {
			ret = append(ret, forma.localSSRC)
		}
	}
	return ret
}

// convert an URL into an address, in particular:
// * add default port
// * handle IPv6 with or without square brackets.
// Adapted from net/http:
// https://cs.opensource.google/go/go/+/refs/tags/go1.20.5:src/net/http/transport.go;l=2747
func canonicalAddr(u *base.URL) string {
	addr := u.Hostname()

	port := u.Port()
	if port == "" {
		if u.Scheme == "rtsp" {
			port = "554"
		} else { // rtsps
			port = "322"
		}
	}

	return net.JoinHostPort(addr, port)
}

func isAnyPort(p int) bool {
	return p == 0 || p == 1
}

func findBaseURL(sd *sdp.SessionDescription, res *base.Response, u *base.URL) (*base.URL, error) {
	// use global control attribute
	if control, ok := sd.Attribute("control"); ok && control != "*" {
		ret, err := base.ParseURL(control)
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

		if strings.HasPrefix(cb[0], "/") {
			// parse as a relative path
			ret, err := base.ParseURL(u.Scheme + "://" + u.Host + cb[0])
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Base: '%v'", cb)
			}

			// add credentials
			ret.User = u.User

			return ret, nil
		}

		ret, err := base.ParseURL(cb[0])
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

type clientAnnounceDataFormat struct {
	localSSRC uint32
}

type clientAnnounceDataMedia struct {
	srtpOutKey []byte
	formats    map[uint8]*clientAnnounceDataFormat
}

func announceDataPickLocalSSRC(
	am *clientAnnounceDataMedia,
	data map[*description.Media]*clientAnnounceDataMedia,
) (uint32, error) {
	var existing []uint32 //nolint:prealloc

	for _, am := range data {
		for _, af := range am.formats {
			existing = append(existing, af.localSSRC)
		}
	}

	for _, af := range am.formats {
		existing = append(existing, af.localSSRC)
	}

	for {
		ssrc, err := randUint32()
		if err != nil {
			return 0, err
		}

		if ssrc != 0 && !slices.Contains(existing, ssrc) {
			return ssrc, nil
		}
	}
}

func generateAnnounceData(
	desc *description.Session,
	secure bool,
) (map[*description.Media]*clientAnnounceDataMedia, error) {
	data := make(map[*description.Media]*clientAnnounceDataMedia)

	for _, medi := range desc.Medias {
		am := &clientAnnounceDataMedia{
			formats: make(map[uint8]*clientAnnounceDataFormat),
		}

		for _, format := range medi.Formats {
			dataFormat := &clientAnnounceDataFormat{}

			var err error
			dataFormat.localSSRC, err = announceDataPickLocalSSRC(am, data)
			if err != nil {
				return nil, err
			}

			am.formats[format.PayloadType()] = dataFormat
		}

		if secure {
			am.srtpOutKey = make([]byte, srtpKeyLength)
			_, err := rand.Read(am.srtpOutKey)
			if err != nil {
				return nil, err
			}
		}

		data[medi] = am
	}

	return data, nil
}

func prepareForAnnounce(
	desc *description.Session,
	announceData map[*description.Media]*clientAnnounceDataMedia,
	secure bool,
) error {
	for i, m := range desc.Medias {
		m.Control = "trackID=" + strconv.FormatInt(int64(i), 10)

		if secure {
			m.Profile = headers.TransportProfileSAVP
			announceDataMedia := announceData[m]

			ssrcs := make([]uint32, len(m.Formats))
			n := 0
			for _, af := range announceDataMedia.formats {
				ssrcs[n] = af.localSSRC
				n++
			}

			// create a temporary Context.
			// Context is needed to extract ROC, but since client has not started streaming,
			// ROC is always zero, therefore a temporary Context can be used.
			srtpCtx := &wrappedSRTPContext{
				key:   announceDataMedia.srtpOutKey,
				ssrcs: ssrcs,
			}
			err := srtpCtx.initialize()
			if err != nil {
				return err
			}

			mikeyMsg, err := mikeyGenerate(srtpCtx)
			if err != nil {
				return err
			}

			m.KeyMgmtMikey = mikeyMsg
		} else {
			m.Profile = headers.TransportProfileAVP
		}
	}

	return nil
}

func supportsGetParameter(header base.Header) bool {
	pub, ok := header["Public"]
	if !ok || len(pub) != 1 {
		return false
	}

	for _, m := range strings.Split(pub[0], ",") {
		if base.Method(strings.Trim(m, " ")) == base.GetParameter {
			return true
		}
	}
	return false
}

func interfaceOfConn(c net.Conn) (*net.Interface, error) {
	var localIP net.IP

	switch addr := c.LocalAddr().(type) {
	case *net.TCPAddr:
		localIP = addr.IP
	case *net.UDPAddr:
		localIP = addr.IP
	default:
		return nil, fmt.Errorf("unknown connection type: %T", c.LocalAddr())
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		var addrs []net.Addr
		addrs, err = iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.Equal(localIP) {
				return &iface, nil
			}
		}
	}

	return nil, fmt.Errorf("no interface found for IP %s", localIP)
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
	url *base.URL
	res chan clientRes
}

type describeReq struct {
	url *base.URL
	res chan clientRes
}

type announceReq struct {
	url  *base.URL
	desc *description.Session
	res  chan clientRes
}

type setupReq struct {
	baseURL  *base.URL
	media    *description.Media
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
	sd  *description.Session // describe only
	res *base.Response
	err error
}

// ClientOnRequestFunc is the prototype of Client.OnRequest.
type ClientOnRequestFunc func(*base.Request)

// ClientOnResponseFunc is the prototype of Client.OnResponse.
type ClientOnResponseFunc func(*base.Response)

// ClientOnTransportSwitchFunc is the prototype of Client.OnTransportSwitch.
type ClientOnTransportSwitchFunc func(err error)

// ClientOnPacketsLostFunc is the prototype of Client.OnPacketsLost.
type ClientOnPacketsLostFunc func(lost uint64)

// ClientOnDecodeErrorFunc is the prototype of Client.OnDecodeError.
type ClientOnDecodeErrorFunc func(err error)

// OnPacketRTPFunc is the prototype of the callback passed to OnPacketRTP().
type OnPacketRTPFunc func(*rtp.Packet)

// OnPacketRTPAnyFunc is the prototype of the callback passed to OnPacketRTP(Any).
type OnPacketRTPAnyFunc func(*description.Media, format.Format, *rtp.Packet)

// OnPacketRTCPFunc is the prototype of the callback passed to OnPacketRTCP().
type OnPacketRTCPFunc func(rtcp.Packet)

// OnPacketRTCPAnyFunc is the prototype of the callback passed to OnPacketRTCPAny().
type OnPacketRTCPAnyFunc func(*description.Media, rtcp.Packet)

// Client is a RTSP client.
type Client struct {
	//
	// Target
	//
	// Scheme. Either "rtsp" or "rtsps".
	Scheme string
	// Host and port.
	Host string

	//
	// RTSP parameters (all optional)
	//
	// timeout of read operations.
	// It defaults to 10 seconds.
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds.
	WriteTimeout time.Duration
	// a TLS configuration to connect to TLS/RTSPS servers.
	// It defaults to nil.
	TLSConfig *tls.Config
	// tunneling method.
	Tunnel Tunnel
	// transport protocol (UDP, Multicast or TCP).
	// If nil, it is chosen automatically (first UDP, then, if it fails, TCP).
	// It defaults to nil.
	Protocol *Protocol
	// enable communication with servers which don't provide UDP server ports
	// or use different server ports than the announced ones.
	// This can be a security issue.
	// It defaults to false.
	AnyPortEnable bool
	// If the client is reading with UDP, it must receive
	// at least a packet within this timeout, otherwise it switches to TCP.
	// It defaults to 3 seconds.
	InitialUDPReadTimeout time.Duration
	// Size of the UDP read buffer.
	// This can be increased to reduce packet losses.
	// It defaults to the operating system default value.
	UDPReadBufferSize int
	// Size of the queue of outgoing packets.
	// It defaults to 256.
	WriteQueueSize int
	// maximum size of outgoing RTP / RTCP packets.
	// This must be less than the UDP MTU (1472 bytes).
	// It defaults to 1472.
	MaxPacketSize int
	// user agent header.
	// It defaults to "gortsplib"
	UserAgent string
	// disable automatic RTCP sender reports.
	DisableRTCPSenderReports bool
	// explicitly request back channels to the server.
	RequestBackChannels bool

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
	// called when sending a request to the server.
	OnRequest ClientOnRequestFunc
	// called when receiving a response from the server.
	OnResponse ClientOnResponseFunc
	// called when receiving a request from the server.
	OnServerRequest ClientOnRequestFunc
	// called when sending a response to the server.
	OnServerResponse ClientOnResponseFunc
	// called when the transport protocol changes.
	OnTransportSwitch ClientOnTransportSwitchFunc
	// called when the client detects lost packets.
	OnPacketsLost ClientOnPacketsLostFunc
	// called when a non-fatal decode error occurs.
	OnDecodeError ClientOnDecodeErrorFunc

	//
	// private
	//

	timeNow              func() time.Time
	senderReportPeriod   time.Duration
	receiverReportPeriod time.Duration
	checkTimeoutPeriod   time.Duration

	ctx                  context.Context
	ctxCancel            func()
	propsMutex           sync.RWMutex
	state                clientState
	nconn                net.Conn
	conn                 *conn.Conn
	session              string
	sender               *auth.Sender
	cseq                 int
	optionsSent          bool
	useGetParameter      bool
	lastDescribeURL      *base.URL
	lastDescribeDesc     *description.Session
	baseURL              *base.URL
	announceData         map[*description.Media]*clientAnnounceDataMedia // record
	setuppedTransport    *SessionTransport
	backChannelSetupped  bool
	stdChannelSetupped   bool
	setuppedMedias       map[*description.Media]*clientMedia
	tcpCallbackByChannel map[int]readFunc
	lastRange            *headers.Range
	checkTimeoutTimer    *time.Timer
	checkTimeoutInitial  bool
	tcpLastFrameTime     *int64
	keepAlivePeriod      time.Duration
	keepAliveTimer       *time.Timer
	closeError           error
	writerMutex          sync.RWMutex
	writer               *asyncprocessor.Processor
	reader               *clientReader
	timeDecoder          *rtptime.GlobalDecoder
	mustClose            bool
	tcpFrame             *base.InterleavedFrame
	tcpBuffer            []byte
	bytesReceived        *uint64
	bytesSent            *uint64

	// in
	chOptions     chan optionsReq
	chDescribe    chan describeReq
	chAnnounce    chan announceReq
	chSetup       chan setupReq
	chPlay        chan playReq
	chRecord      chan recordReq
	chPause       chan pauseReq
	chResponse    chan *base.Response
	chRequest     chan *base.Request
	chReadError   chan error
	chWriterError chan error

	// out
	done chan struct{}
}

// Start initializes the connection to a server.
func (c *Client) Start() error {
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
	if c.WriteQueueSize == 0 {
		c.WriteQueueSize = 256
	} else if (c.WriteQueueSize & (c.WriteQueueSize - 1)) != 0 {
		return fmt.Errorf("WriteQueueSize must be a power of two")
	}
	if c.MaxPacketSize == 0 {
		c.MaxPacketSize = udpMaxPayloadSize
	} else if c.MaxPacketSize > udpMaxPayloadSize {
		return fmt.Errorf("MaxPacketSize must be less than %d", udpMaxPayloadSize)
	}
	if c.UserAgent == "" {
		c.UserAgent = clientUserAgent
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
	if c.OnServerRequest == nil {
		c.OnServerRequest = func(*base.Request) {
		}
	}
	if c.OnServerResponse == nil {
		c.OnServerResponse = func(*base.Response) {
		}
	}
	if c.OnTransportSwitch == nil {
		c.OnTransportSwitch = func(err error) {
			log.Println(err.Error())
		}
	}
	if c.OnPacketsLost == nil {
		c.OnPacketsLost = func(lost uint64) {
			log.Printf("%d RTP %s lost",
				lost,
				func() string {
					if lost == 1 {
						return "packet"
					}
					return "packets"
				}())
		}
	}
	if c.OnDecodeError == nil {
		c.OnDecodeError = func(err error) {
			log.Println(err.Error())
		}
	}

	// private
	if c.timeNow == nil {
		c.timeNow = time.Now
	}
	if c.senderReportPeriod == 0 {
		c.senderReportPeriod = 10 * time.Second
	}
	if c.receiverReportPeriod == 0 {
		// some cameras require a maximum of 5secs between keepalives
		c.receiverReportPeriod = 5 * time.Second
	}
	if c.checkTimeoutPeriod == 0 {
		c.checkTimeoutPeriod = 1 * time.Second
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	c.ctx = ctx
	c.ctxCancel = ctxCancel
	c.checkTimeoutTimer = emptyTimer()
	c.keepAlivePeriod = 30 * time.Second
	c.keepAliveTimer = emptyTimer()
	c.bytesReceived = new(uint64)
	c.bytesSent = new(uint64)

	c.chOptions = make(chan optionsReq)
	c.chDescribe = make(chan describeReq)
	c.chAnnounce = make(chan announceReq)
	c.chSetup = make(chan setupReq)
	c.chPlay = make(chan playReq)
	c.chRecord = make(chan recordReq)
	c.chPause = make(chan pauseReq)
	c.chResponse = make(chan *base.Response)
	c.chRequest = make(chan *base.Request)
	c.chReadError = make(chan error)
	c.chWriterError = make(chan error)
	c.done = make(chan struct{})

	go c.run()

	return nil
}

// StartRecording connects to the address and starts publishing given media.
func (c *Client) StartRecording(address string, desc *description.Session) error {
	u, err := base.ParseURL(address)
	if err != nil {
		return err
	}

	c.Scheme = u.Scheme
	c.Host = u.Host

	err = c.Start()
	if err != nil {
		return err
	}

	_, err = c.Announce(u, desc)
	if err != nil {
		c.Close()
		return err
	}

	err = c.SetupAll(u, desc.Medias)
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

// Close closes all client resources and waits for them to exit.
func (c *Client) Close() {
	c.ctxCancel()
	<-c.done
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
		case req := <-c.chOptions:
			res, err := c.doOptions(req.url)
			req.res <- clientRes{res: res, err: err}

			if c.mustClose {
				return err
			}

		case req := <-c.chDescribe:
			sd, res, err := c.doDescribe(req.url)
			req.res <- clientRes{sd: sd, res: res, err: err}

			if c.mustClose {
				return err
			}

		case req := <-c.chAnnounce:
			res, err := c.doAnnounce(req.url, req.desc)
			req.res <- clientRes{res: res, err: err}

			if c.mustClose {
				return err
			}

		case req := <-c.chSetup:
			res, err := c.doSetup(req.baseURL, req.media, req.rtpPort, req.rtcpPort)
			req.res <- clientRes{res: res, err: err}

			if c.mustClose {
				return err
			}

		case req := <-c.chPlay:
			res, err := c.doPlay(req.ra)
			req.res <- clientRes{res: res, err: err}

			if c.mustClose {
				return err
			}

		case req := <-c.chRecord:
			res, err := c.doRecord()
			req.res <- clientRes{res: res, err: err}

			if c.mustClose {
				return err
			}

		case req := <-c.chPause:
			res, err := c.doPause()
			req.res <- clientRes{res: res, err: err}

			if c.mustClose {
				return err
			}

		case <-c.checkTimeoutTimer.C:
			err := c.doCheckTimeout()
			if err != nil {
				return err
			}
			c.checkTimeoutTimer = time.NewTimer(c.checkTimeoutPeriod)

		case <-c.keepAliveTimer.C:
			err := c.doKeepAlive()
			if err != nil {
				return err
			}
			c.keepAliveTimer = time.NewTimer(c.keepAlivePeriod)

		case res := <-c.chResponse:
			c.OnResponse(res)
			// these are responses to keepalives, ignore them.

		case req := <-c.chRequest:
			err := c.handleServerRequest(req)
			if err != nil {
				return err
			}

		case err := <-c.chReadError:
			c.reader.close()
			c.reader = nil
			return err

		case err := <-c.chWriterError:
			return err

		case <-c.ctx.Done():
			return liberrors.ErrClientTerminated{}
		}
	}
}

func (c *Client) waitResponse(requestCseqStr string) (*base.Response, error) {
	t := time.NewTimer(c.ReadTimeout)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			return nil, liberrors.ErrClientRequestTimedOut{}

		case req := <-c.chRequest:
			err := c.handleServerRequest(req)
			if err != nil {
				return nil, err
			}

		case res := <-c.chResponse:
			c.OnResponse(res)

			// accept response if CSeq equals request CSeq, or if CSeq is not present
			if cseq, ok := res.Header["CSeq"]; !ok || len(cseq) != 1 || strings.TrimSpace(cseq[0]) == requestCseqStr {
				return res, nil
			}

		case err := <-c.chReadError:
			c.reader.close()
			c.reader = nil
			return nil, err

		case <-c.ctx.Done():
			return nil, liberrors.ErrClientTerminated{}
		}
	}
}

func (c *Client) handleServerRequest(req *base.Request) error {
	c.OnServerRequest(req)

	if req.Method != base.Options {
		return liberrors.ErrClientUnhandledMethod{Method: req.Method}
	}

	h := base.Header{
		"User-Agent": base.HeaderValue{c.UserAgent},
	}

	if cseq, ok := req.Header["CSeq"]; ok {
		h["CSeq"] = cseq
	}

	res := &base.Response{
		StatusCode: base.StatusOK,
		Header:     h,
	}

	c.OnServerResponse(res)

	c.nconn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
	return c.conn.WriteResponse(res)
}

func (c *Client) doClose() {
	if c.state == clientStatePlay || c.state == clientStateRecord {
		c.destroyWriter()
		c.stopTransportRoutines()
	}

	if c.nconn != nil && c.baseURL != nil {
		header := base.Header{}

		if c.backChannelSetupped {
			header["Require"] = base.HeaderValue{"www.onvif.org/ver20/backchannel"}
		}

		c.do(&base.Request{ //nolint:errcheck
			Method: base.Teardown,
			URL:    c.baseURL,
			Header: header,
		}, true)
	}

	if c.reader != nil {
		c.nconn.Close()
		c.reader.close()
		c.reader = nil
		c.nconn = nil
		c.conn = nil
	} else if c.nconn != nil {
		c.nconn.Close()
		c.nconn = nil
		c.conn = nil
	}

	for _, cm := range c.setuppedMedias {
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
	c.setuppedTransport = nil
	c.backChannelSetupped = false
	c.stdChannelSetupped = false
	c.setuppedMedias = nil
	c.tcpCallbackByChannel = nil
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
	c.OnTransportSwitch(liberrors.ErrClientSwitchToTCP{})

	prevBaseURL := c.baseURL
	prevMedias := c.setuppedMedias

	c.reset()

	c.setuppedTransport = &SessionTransport{
		Protocol: ProtocolTCP,
	}

	// some Hikvision cameras require a describe before a setup
	_, _, err := c.doDescribe(c.lastDescribeURL)
	if err != nil {
		return err
	}

	for i, cm := range prevMedias {
		_, err = c.doSetup(prevBaseURL, cm.media, 0, 0)
		if err != nil {
			return err
		}

		c.setuppedMedias[i].onPacketRTCP = cm.onPacketRTCP
		for j, tr := range cm.formats {
			c.setuppedMedias[i].formats[j].onPacketRTP = tr.onPacketRTP
		}
	}

	_, err = c.doPlay(c.lastRange)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) startTransportRoutines() {
	c.timeDecoder = &rtptime.GlobalDecoder{}
	c.timeDecoder.Initialize()

	for _, cm := range c.setuppedMedias {
		cm.start()
	}

	if c.setuppedTransport.Protocol == ProtocolTCP {
		c.tcpFrame = &base.InterleavedFrame{}
		c.tcpBuffer = make([]byte, c.MaxPacketSize+4)
	}

	// always enable keepalives unless we are recording with TCP
	if c.state == clientStatePlay || c.setuppedTransport.Protocol != ProtocolTCP {
		c.keepAliveTimer = time.NewTimer(c.keepAlivePeriod)
	}

	if c.state == clientStatePlay && c.stdChannelSetupped {
		switch c.setuppedTransport.Protocol {
		case ProtocolUDP:
			c.checkTimeoutTimer = time.NewTimer(c.InitialUDPReadTimeout)
			c.checkTimeoutInitial = true

		case ProtocolUDPMulticast:
			c.checkTimeoutTimer = time.NewTimer(c.checkTimeoutPeriod)

		default: // TCP
			c.checkTimeoutTimer = time.NewTimer(c.checkTimeoutPeriod)
			v := c.timeNow().Unix()
			c.tcpLastFrameTime = &v
		}
	}

	if c.setuppedTransport.Protocol == ProtocolTCP {
		c.reader.setAllowInterleavedFrames(true)
	}
}

func (c *Client) stopTransportRoutines() {
	if c.reader != nil {
		c.reader.setAllowInterleavedFrames(false)
	}

	c.checkTimeoutTimer = emptyTimer()
	c.keepAliveTimer = emptyTimer()

	for _, cm := range c.setuppedMedias {
		cm.stop()
	}

	c.timeDecoder = nil
}

func (c *Client) createWriter() {
	c.writerMutex.Lock()

	c.writer = &asyncprocessor.Processor{
		BufferSize: func() int {
			if c.state == clientStateRecord || c.backChannelSetupped {
				return c.WriteQueueSize
			}

			// when reading, buffer is only used to send RTCP receiver reports,
			// that are much smaller than RTP packets and are sent at a fixed interval.
			// decrease RAM consumption by allocating less buffers.
			return 8
		}(),
		OnError: func(ctx context.Context, err error) {
			select {
			case <-ctx.Done():
			case <-c.ctx.Done():
			case c.chWriterError <- err:
			}
		},
	}
	c.writer.Initialize()

	c.writerMutex.Unlock()
}

func (c *Client) startWriter() {
	c.writer.Start()
}

func (c *Client) destroyWriter() {
	c.writer.Close()

	c.writerMutex.Lock()
	c.writer = nil
	c.writerMutex.Unlock()
}

func (c *Client) connOpen() error {
	if c.nconn != nil {
		return nil
	}

	if c.Scheme != "rtsp" && c.Scheme != "rtsps" {
		return liberrors.ErrClientUnsupportedScheme{Scheme: c.Scheme}
	}

	dialCtx, dialCtxCancel := context.WithTimeout(c.ctx, c.ReadTimeout)
	defer dialCtxCancel()

	addr := canonicalAddr(&base.URL{
		Scheme: c.Scheme,
		Host:   c.Host,
	})

	var tlsConfig *tls.Config
	if c.Scheme == "rtsps" {
		tlsConfig = c.TLSConfig
		if tlsConfig == nil {
			host, _, _ := net.SplitHostPort(addr)
			tlsConfig = &tls.Config{
				ServerName: host,
			}
		}
	}

	var nconn net.Conn

	switch c.Tunnel {
	case TunnelHTTP:
		var err error
		nconn, err = newClientTunnelHTTP(dialCtx, c.DialContext, addr, tlsConfig)
		if err != nil {
			return err
		}

	case TunnelWebSocket:
		var err error
		nconn, err = newClientTunnelWebSocket(dialCtx, c.DialContext, addr, tlsConfig)
		if err != nil {
			return err
		}

	default:
		var err error
		nconn, err = c.DialContext(dialCtx, "tcp", addr)
		if err != nil {
			return err
		}

		if tlsConfig != nil {
			nconn = tls.Client(nconn, tlsConfig)
		}
	}

	c.nconn = nconn
	bc := bytecounter.New(c.nconn, c.bytesReceived, c.bytesSent)
	c.conn = conn.NewConn(bufio.NewReader(bc), bc)
	c.reader = &clientReader{
		c: c,
	}
	c.reader.start()

	return nil
}

func (c *Client) do(req *base.Request, skipResponse bool) (*base.Response, error) {
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
	cseqStr := strconv.FormatInt(int64(c.cseq), 10)
	req.Header["CSeq"] = base.HeaderValue{cseqStr}

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

	res, err := c.waitResponse(cseqStr)
	if err != nil {
		c.mustClose = true
		return nil, err
	}

	// get session from response
	if v, ok := res.Header["Session"]; ok {
		var sx headers.Session
		err = sx.Unmarshal(v)
		if err != nil {
			return nil, liberrors.ErrClientSessionHeaderInvalid{Err: err}
		}
		c.session = sx.Session

		if sx.Timeout != nil && *sx.Timeout > 0 {
			c.keepAlivePeriod = time.Duration(*sx.Timeout) * time.Second * 8 / 10
		}
	}

	// send request again with authentication
	if res.StatusCode == base.StatusUnauthorized && req.URL.User != nil && c.sender == nil {
		pass, _ := req.URL.User.Password()
		user := req.URL.User.Username()

		sender := &auth.Sender{
			WWWAuth: res.Header["WWW-Authenticate"],
			User:    user,
			Pass:    pass,
		}
		err = sender.Initialize()
		if err != nil {
			return nil, liberrors.ErrClientAuthSetup{Err: err}
		}
		c.sender = sender

		return c.do(req, skipResponse)
	}

	return res, nil
}

func (c *Client) atLeastOneUDPPacketHasBeenReceived() bool {
	for _, ct := range c.setuppedMedias {
		lft := atomic.LoadInt64(ct.udpRTPListener.lastPacketTime)
		if lft != 0 {
			return true
		}

		lft = atomic.LoadInt64(ct.udpRTCPListener.lastPacketTime)
		if lft != 0 {
			return true
		}
	}
	return false
}

func (c *Client) isInUDPTimeout() bool {
	now := c.timeNow()
	for _, ct := range c.setuppedMedias {
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
}

func (c *Client) isInTCPTimeout() bool {
	now := c.timeNow()
	lft := time.Unix(atomic.LoadInt64(c.tcpLastFrameTime), 0)
	return now.Sub(lft) >= c.ReadTimeout
}

func (c *Client) doCheckTimeout() error {
	if c.setuppedTransport.Protocol == ProtocolUDP ||
		c.setuppedTransport.Protocol == ProtocolUDPMulticast {
		if c.checkTimeoutInitial && !c.backChannelSetupped && c.Protocol == nil {
			c.checkTimeoutInitial = false

			if !c.atLeastOneUDPPacketHasBeenReceived() {
				err := c.trySwitchingProtocol()
				if err != nil {
					return err
				}
			}
		} else if c.isInUDPTimeout() {
			return liberrors.ErrClientUDPTimeout{}
		}
	} else if c.isInTCPTimeout() {
		return liberrors.ErrClientTCPTimeout{}
	}

	return nil
}

func (c *Client) doKeepAlive() error {
	// some cameras do not reply to keepalives, do not wait for responses.
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
	}, true)
	return err
}

func (c *Client) doOptions(u *base.URL) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial:   {},
		clientStatePrePlay:   {},
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	err = c.connOpen()
	if err != nil {
		return nil, err
	}

	res, err := c.do(&base.Request{
		Method: base.Options,
		URL:    u,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		// since this method is not implemented by every RTSP server,
		// return an error only if status code is not 404
		if res.StatusCode == base.StatusNotFound {
			return res, nil
		}
		return nil, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	c.optionsSent = true
	c.useGetParameter = supportsGetParameter(res.Header)

	return res, nil
}

// Options sends an OPTIONS request.
func (c *Client) Options(u *base.URL) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.chOptions <- optionsReq{url: u, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.done:
		return nil, c.closeError
	}
}

func (c *Client) doDescribe(u *base.URL) (*description.Session, *base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial:   {},
		clientStatePrePlay:   {},
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, nil, err
	}

	err = c.connOpen()
	if err != nil {
		return nil, nil, err
	}

	header := base.Header{
		"Accept": base.HeaderValue{"application/sdp"},
	}

	if c.RequestBackChannels {
		header["Require"] = base.HeaderValue{"www.onvif.org/ver20/backchannel"}
	}

	res, err := c.do(&base.Request{
		Method: base.Describe,
		URL:    u,
		Header: header,
	}, false)
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode != base.StatusOK {
		// redirect
		if res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {
			c.reset()

			var ru *base.URL
			ru, err = base.ParseURL(res.Header["Location"][0])
			if err != nil {
				return nil, nil, err
			}

			if c.Scheme == "rtsps" && ru.Scheme != "rtsps" {
				return nil, nil, fmt.Errorf("connection cannot be downgraded from RTSPS to RTSP")
			}

			if u.User != nil {
				ru.User = u.User
			}

			c.Scheme = ru.Scheme
			c.Host = ru.Host

			return c.doDescribe(ru)
		}

		return nil, res, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	ct, ok := res.Header["Content-Type"]
	if !ok || len(ct) != 1 {
		return nil, nil, liberrors.ErrClientContentTypeMissing{}
	}

	// strip encoding information from Content-Type header
	ct = base.HeaderValue{strings.Split(ct[0], ";")[0]}

	if ct[0] != "application/sdp" {
		return nil, nil, liberrors.ErrClientContentTypeUnsupported{CT: ct}
	}

	var ssd sdp.SessionDescription
	err = ssd.Unmarshal(res.Body)
	if err != nil {
		return nil, nil, liberrors.ErrClientSDPInvalid{Err: err}
	}

	var desc description.Session
	err = desc.Unmarshal(&ssd)
	if err != nil {
		return nil, nil, liberrors.ErrClientSDPInvalid{Err: err}
	}

	baseURL, err := findBaseURL(&ssd, res, u)
	if err != nil {
		return nil, nil, err
	}
	desc.BaseURL = baseURL

	c.lastDescribeURL = u
	c.lastDescribeDesc = &desc

	return &desc, res, nil
}

// Describe sends a DESCRIBE request.
func (c *Client) Describe(u *base.URL) (*description.Session, *base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.chDescribe <- describeReq{url: u, res: cres}:
		res := <-cres
		return res.sd, res.res, res.err

	case <-c.done:
		return nil, nil, c.closeError
	}
}

func (c *Client) doAnnounce(u *base.URL, desc *description.Session) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial: {},
	})
	if err != nil {
		return nil, err
	}

	if c.Protocol != nil && *c.Protocol == ProtocolUDPMulticast {
		return nil, fmt.Errorf("recording with UDP multicast is not supported")
	}

	err = c.connOpen()
	if err != nil {
		return nil, err
	}

	// Determine secure flag: TCP+RTSPS depends on media profile, others depend on scheme
	var secure bool

	// Determine secure flag: TCP+RTSPS depends on media profile, others depend on scheme
	if c.Protocol != nil && *c.Protocol == ProtocolTCP && c.Scheme == "rtsps" {
		// Check for all medias: if any media uses a secure profile, then secure is true
		for _, medi := range desc.Medias {
			if isSecure(medi.Profile) {
				secure = true
				break
			}
		}
	} else {
		secure = c.Scheme == "rtsps"
	}

	announceData, err := generateAnnounceData(desc, secure)
	if err != nil {
		return nil, err
	}

	err = prepareForAnnounce(desc, announceData, secure)
	if err != nil {
		return nil, err
	}

	byts, err := desc.Marshal()
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
	}, false)
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
	c.announceData = announceData

	return res, nil
}

// Announce sends an ANNOUNCE request.
func (c *Client) Announce(u *base.URL, desc *description.Session) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.chAnnounce <- announceReq{url: u, desc: desc, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.done:
		return nil, c.closeError
	}
}

func (c *Client) doSetup(
	baseURL *base.URL,
	medi *description.Media,
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

	err = c.connOpen()
	if err != nil {
		return nil, err
	}

	if c.baseURL != nil && *baseURL != *c.baseURL {
		return nil, liberrors.ErrClientCannotSetupMediasDifferentURLs{}
	}

	th := headers.Transport{}

	// when playing, omit mode, since it causes errors with some servers.
	if c.state == clientStatePreRecord {
		v := headers.TransportModeRecord
		th.Mode = &v
	}

	var protocol Protocol

	switch {
	// use transport from previous SETUP calls
	case c.setuppedTransport != nil:
		protocol = c.setuppedTransport.Protocol
		th.Profile = c.setuppedTransport.Profile

	// use transport from config, secure flag from server
	case c.Protocol != nil:
		protocol = *c.Protocol
		if isSecure(medi.Profile) && c.Scheme == "rtsps" {
			th.Profile = headers.TransportProfileSAVP
		} else {
			th.Profile = headers.TransportProfileAVP
		}

	default:
		if isSecure(medi.Profile) && c.Scheme == "rtsps" {
			th.Profile = headers.TransportProfileSAVP
		} else {
			th.Profile = headers.TransportProfileAVP
		}

		// try
		// - UDP if unencrypted or secure is supported by server
		// - otherwise, TCP
		if c.Tunnel == TunnelNone && (th.Profile == headers.TransportProfileSAVP || c.Scheme == "rtsp") {
			protocol = ProtocolUDP
		} else {
			protocol = ProtocolTCP
		}
	}

	var localSSRCs map[uint8]uint32

	if c.state == clientStatePreRecord {
		localSSRCs = make(map[uint8]uint32)
		for forma, data := range c.announceData[medi].formats {
			localSSRCs[forma] = data.localSSRC
		}
	} else {
		localSSRCs, err = generateLocalSSRCs(
			clientExtractExistingSSRCs(c.setuppedMedias),
			medi.Formats,
		)
		if err != nil {
			return nil, err
		}
	}

	var udpRTPListener *clientUDPListener
	var udpRTCPListener *clientUDPListener
	var tcpChannel int
	var srtpInCtx *wrappedSRTPContext
	var srtpOutCtx *wrappedSRTPContext

	defer func() {
		if udpRTPListener != nil {
			udpRTPListener.close()
		}
		if udpRTCPListener != nil {
			udpRTCPListener.close()
		}
	}()

	switch protocol {
	case ProtocolUDP, ProtocolUDPMulticast:
		if c.Scheme == "rtsps" && !isSecure(th.Profile) {
			return nil, fmt.Errorf("unable to setup secure UDP")
		}

		th.Protocol = headers.TransportProtocolUDP

		if protocol == ProtocolUDP {
			if (rtpPort == 0 && rtcpPort != 0) ||
				(rtpPort != 0 && rtcpPort == 0) {
				return nil, liberrors.ErrClientUDPPortsZero{}
			}

			if rtpPort != 0 && rtcpPort != (rtpPort+1) {
				return nil, liberrors.ErrClientUDPPortsNotConsecutive{}
			}

			udpRTPListener, udpRTCPListener, err = createUDPListenerPair(
				c,
				false,
				nil,
				net.JoinHostPort("", strconv.FormatInt(int64(rtpPort), 10)),
				net.JoinHostPort("", strconv.FormatInt(int64(rtcpPort), 10)),
			)
			if err != nil {
				return nil, err
			}

			v1 := headers.TransportDeliveryUnicast
			th.Delivery = &v1
			th.ClientPorts = &[2]int{udpRTPListener.port(), udpRTCPListener.port()}
		} else {
			v1 := headers.TransportDeliveryMulticast
			th.Delivery = &v1
		}

	case ProtocolTCP:
		v1 := headers.TransportDeliveryUnicast
		th.Delivery = &v1
		th.Protocol = headers.TransportProtocolTCP
		ch := c.findFreeChannelPair()
		th.InterleavedIDs = &[2]int{ch, ch + 1}
	}

	mediaURL, err := medi.URL(baseURL)
	if err != nil {
		return nil, err
	}

	header := base.Header{
		"Transport": th.Marshal(),
	}

	if medi.IsBackChannel {
		if !c.RequestBackChannels {
			return nil, fmt.Errorf("we are setupping a back channel but we did not request back channels")
		}

		header["Require"] = base.HeaderValue{"www.onvif.org/ver20/backchannel"}
	}

	if isSecure(th.Profile) {
		var srtpOutKey []byte

		if c.state == clientStatePreRecord {
			srtpOutKey = c.announceData[medi].srtpOutKey
		} else {
			srtpOutKey = make([]byte, srtpKeyLength)
			_, err = rand.Read(srtpOutKey)
			if err != nil {
				return nil, err
			}
		}

		srtpOutCtx = &wrappedSRTPContext{
			key:   srtpOutKey,
			ssrcs: ssrcsMapToList(localSSRCs),
		}
		err = srtpOutCtx.initialize()
		if err != nil {
			return nil, err
		}

		var mikeyMsg *mikey.Message
		mikeyMsg, err = mikeyGenerate(srtpOutCtx)
		if err != nil {
			return nil, err
		}

		var enc base.HeaderValue
		enc, err = headers.KeyMgmt{
			URL:          mediaURL.String(),
			MikeyMessage: mikeyMsg,
		}.Marshal()
		if err != nil {
			return nil, err
		}

		header["KeyMgmt"] = enc
	}

	res, err := c.do(&base.Request{
		Method: base.Setup,
		URL:    mediaURL,
		Header: header,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		// switch transport automatically
		if res.StatusCode == base.StatusUnsupportedTransport &&
			c.setuppedTransport == nil && c.Protocol == nil {
			c.OnTransportSwitch(liberrors.ErrClientSwitchToTCP2{})
			c.setuppedTransport = &SessionTransport{
				Protocol: ProtocolTCP,
				Profile:  th.Profile,
			}

			return c.doSetup(baseURL, medi, 0, 0)
		}

		return nil, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	var thRes headers.Transport
	err = thRes.Unmarshal(res.Header["Transport"])
	if err != nil {
		return nil, liberrors.ErrClientTransportHeaderInvalid{Err: err}
	}

	switch protocol {
	case ProtocolUDP, ProtocolUDPMulticast:
		if thRes.Protocol == headers.TransportProtocolTCP {
			// switch transport automatically
			if c.setuppedTransport == nil && c.Protocol == nil {
				c.OnTransportSwitch(liberrors.ErrClientSwitchToTCP2{})

				c.baseURL = baseURL

				c.reset()

				c.setuppedTransport = &SessionTransport{
					Protocol: ProtocolTCP,
					Profile:  th.Profile,
				}

				// some Hikvision cameras require a describe before a setup
				_, _, err = c.doDescribe(c.lastDescribeURL)
				if err != nil {
					return nil, err
				}

				return c.doSetup(baseURL, medi, 0, 0)
			}

			return nil, liberrors.ErrClientServerRequestedTCP{}
		}
	}

	switch protocol {
	case ProtocolUDP:
		if thRes.Delivery != nil && *thRes.Delivery != headers.TransportDeliveryUnicast {
			return nil, liberrors.ErrClientTransportHeaderInvalidDelivery{}
		}

		serverPortsValid := thRes.ServerPorts != nil && !isAnyPort(thRes.ServerPorts[0]) && !isAnyPort(thRes.ServerPorts[1])

		if (c.state == clientStatePreRecord || !c.AnyPortEnable) && !serverPortsValid {
			return nil, liberrors.ErrClientServerPortsNotProvided{}
		}

		var remoteIP net.IP
		if thRes.Source2 != nil {
			if ip := net.ParseIP(*thRes.Source2); ip != nil {
				remoteIP = ip
			} else {
				var addr *net.UDPAddr
				addr, err = net.ResolveUDPAddr("udp", *thRes.Source2)
				if err != nil {
					return nil, fmt.Errorf("unable to solve source host: %w", err)
				}
				remoteIP = addr.IP
			}
		} else {
			remoteIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		}

		if serverPortsValid {
			if !c.AnyPortEnable {
				udpRTPListener.readPort = thRes.ServerPorts[0]
			}
			udpRTPListener.writeAddr = &net.UDPAddr{
				IP:   remoteIP,
				Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
				Port: thRes.ServerPorts[0],
			}
		}
		udpRTPListener.readIP = remoteIP

		if serverPortsValid {
			if !c.AnyPortEnable {
				udpRTCPListener.readPort = thRes.ServerPorts[1]
			}
			udpRTCPListener.writeAddr = &net.UDPAddr{
				IP:   remoteIP,
				Zone: c.nconn.RemoteAddr().(*net.TCPAddr).Zone,
				Port: thRes.ServerPorts[1],
			}
		}
		udpRTCPListener.readIP = remoteIP

	case ProtocolUDPMulticast:
		if thRes.Delivery == nil || *thRes.Delivery != headers.TransportDeliveryMulticast {
			return nil, liberrors.ErrClientTransportHeaderInvalidDelivery{}
		}

		var remoteIP net.IP
		if thRes.Source2 != nil {
			if ip := net.ParseIP(*thRes.Source2); ip != nil {
				remoteIP = ip
			} else {
				var addr *net.UDPAddr
				addr, err = net.ResolveUDPAddr("udp", *thRes.Source2)
				if err != nil {
					return nil, fmt.Errorf("unable to solve source host: %w", err)
				}
				remoteIP = addr.IP
			}
		} else {
			remoteIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		}

		var destIP net.IP
		if thRes.Destination2 == nil {
			return nil, liberrors.ErrClientTransportHeaderNoDestination{}
		}
		if ip := net.ParseIP(*thRes.Destination2); ip != nil {
			destIP = ip
		} else {
			var addr *net.UDPAddr
			addr, err = net.ResolveUDPAddr("udp", *thRes.Destination2)
			if err != nil {
				return nil, fmt.Errorf("unable to solve destination host: %w", err)
			}
			destIP = addr.IP
		}

		if thRes.Ports == nil {
			return nil, liberrors.ErrClientTransportHeaderNoPorts{}
		}

		var intf *net.Interface
		intf, err = interfaceOfConn(c.nconn)
		if err != nil {
			return nil, err
		}

		udpRTPListener, udpRTCPListener, err = createUDPListenerPair(
			c,
			true,
			intf,
			net.JoinHostPort(destIP.String(), strconv.FormatInt(int64(thRes.Ports[0]), 10)),
			net.JoinHostPort(destIP.String(), strconv.FormatInt(int64(thRes.Ports[1]), 10)),
		)
		if err != nil {
			return nil, err
		}

		udpRTPListener.readIP = remoteIP
		udpRTPListener.readPort = thRes.Ports[0]
		udpRTPListener.writeAddr = &net.UDPAddr{
			IP:   remoteIP,
			Port: thRes.Ports[0],
		}

		udpRTCPListener.readIP = remoteIP
		udpRTCPListener.readPort = thRes.Ports[1]
		udpRTCPListener.writeAddr = &net.UDPAddr{
			IP:   remoteIP,
			Port: thRes.Ports[1],
		}

	case ProtocolTCP:
		if thRes.Protocol != headers.TransportProtocolTCP {
			return nil, liberrors.ErrClientServerRequestedUDP{}
		}

		if thRes.Delivery != nil && *thRes.Delivery != headers.TransportDeliveryUnicast {
			return nil, liberrors.ErrClientTransportHeaderInvalidDelivery{}
		}

		if thRes.InterleavedIDs == nil {
			return nil, liberrors.ErrClientTransportHeaderNoInterleavedIDs{}
		}

		if (thRes.InterleavedIDs[0] + 1) != thRes.InterleavedIDs[1] {
			return nil, liberrors.ErrClientTransportHeaderInvalidInterleavedIDs{}
		}

		if c.isChannelPairInUse(thRes.InterleavedIDs[0]) {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrClientTransportHeaderInterleavedIDsInUse{}
		}

		tcpChannel = thRes.InterleavedIDs[0]
	}

	if thRes.Profile != th.Profile {
		return nil, fmt.Errorf("returned profile does not match requested profile")
	}

	if isSecure(th.Profile) {
		var mikeyMsg *mikey.Message

		// extract key-mgmt from (in order of priority):
		// - response
		// - media SDP attributes
		// - session SDP attributes
		switch {
		case res.Header["KeyMgmt"] != nil:
			var keyMgmt headers.KeyMgmt
			err = keyMgmt.Unmarshal(res.Header["KeyMgmt"])
			if err != nil {
				return nil, err
			}
			mikeyMsg = keyMgmt.MikeyMessage

		case medi.KeyMgmtMikey != nil:
			mikeyMsg = medi.KeyMgmtMikey

		case c.lastDescribeDesc.KeyMgmtMikey != nil:
			mikeyMsg = c.lastDescribeDesc.KeyMgmtMikey

		default:
			return nil, fmt.Errorf("server did not provide key-mgmt data in any supported way")
		}

		srtpInCtx, err = mikeyToContext(mikeyMsg)
		if err != nil {
			return nil, err
		}
	}

	cm := &clientMedia{
		c:               c,
		media:           medi,
		secure:          isSecure(th.Profile),
		udpRTPListener:  udpRTPListener,
		udpRTCPListener: udpRTCPListener,
		tcpChannel:      tcpChannel,
		localSSRCs:      localSSRCs,
		srtpInCtx:       srtpInCtx,
		srtpOutCtx:      srtpOutCtx,
	}
	cm.initialize()

	udpRTPListener = nil
	udpRTCPListener = nil

	c.propsMutex.Lock()

	if c.setuppedMedias == nil {
		c.setuppedMedias = make(map[*description.Media]*clientMedia)
	}
	c.setuppedMedias[medi] = cm

	c.baseURL = baseURL
	c.setuppedTransport = &SessionTransport{
		Protocol: protocol,
		Profile:  th.Profile,
	}

	c.propsMutex.Unlock()

	if medi.IsBackChannel {
		c.backChannelSetupped = true
	} else {
		c.stdChannelSetupped = true
	}

	if c.state == clientStateInitial {
		c.state = clientStatePrePlay
	}

	return res, nil
}

func (c *Client) isChannelPairInUse(channel int) bool {
	for _, cm := range c.setuppedMedias {
		if (cm.tcpChannel+1) == channel || cm.tcpChannel == channel || cm.tcpChannel == (channel+1) {
			return true
		}
	}
	return false
}

func (c *Client) findFreeChannelPair() int {
	for i := 0; ; i += 2 { // prefer even channels
		if !c.isChannelPairInUse(i) {
			return i
		}
	}
}

// Setup sends a SETUP request.
// rtpPort and rtcpPort are used only if transport is UDP.
// if rtpPort and rtcpPort are zero, they are chosen automatically.
func (c *Client) Setup(
	baseURL *base.URL,
	media *description.Media,
	rtpPort int,
	rtcpPort int,
) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.chSetup <- setupReq{
		baseURL:  baseURL,
		media:    media,
		rtpPort:  rtpPort,
		rtcpPort: rtcpPort,
		res:      cres,
	}:
		res := <-cres
		return res.res, res.err

	case <-c.done:
		return nil, c.closeError
	}
}

// SetupAll setups all the given medias.
func (c *Client) SetupAll(baseURL *base.URL, medias []*description.Media) error {
	for _, m := range medias {
		_, err := c.Setup(baseURL, m, 0, 0)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) doPlay(ra *headers.Range) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	c.state = clientStatePlay
	c.startTransportRoutines()
	c.createWriter()

	// Range is mandatory in Parrot Streaming Server
	if ra == nil {
		ra = &headers.Range{
			Value: &headers.RangeNPT{
				Start: 0,
			},
		}
	}

	header := base.Header{
		"Range": ra.Marshal(),
	}

	if c.backChannelSetupped {
		header["Require"] = base.HeaderValue{"www.onvif.org/ver20/backchannel"}
	}

	// when protocol is UDP,
	// open the firewall by sending empty packets to the remote part.
	// do this before sending the PLAY request.
	if c.setuppedTransport.Protocol == ProtocolUDP {
		for _, cm := range c.setuppedMedias {
			if !cm.media.IsBackChannel && cm.udpRTPListener.writeAddr != nil {
				buf, _ := (&rtp.Packet{Header: rtp.Header{Version: 2}}).Marshal()
				if cm.srtpOutCtx != nil {
					encr := make([]byte, cm.c.MaxPacketSize)
					encr, err = cm.srtpOutCtx.encryptRTP(encr, buf, nil)
					if err != nil {
						return nil, err
					}
					buf = encr
				}
				err = cm.udpRTPListener.write(buf)
				if err != nil {
					return nil, err
				}

				buf, _ = (&rtcp.ReceiverReport{}).Marshal()
				if cm.srtpOutCtx != nil {
					encr := make([]byte, cm.c.MaxPacketSize)
					encr, err = cm.srtpOutCtx.encryptRTCP(encr, buf, nil)
					if err != nil {
						return nil, err
					}
					buf = encr
				}
				err = cm.udpRTCPListener.write(buf)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	res, err := c.do(&base.Request{
		Method: base.Play,
		URL:    c.baseURL,
		Header: header,
	}, false)
	if err != nil {
		c.destroyWriter()
		c.stopTransportRoutines()
		c.state = clientStatePrePlay
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		c.destroyWriter()
		c.stopTransportRoutines()
		c.state = clientStatePrePlay
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.startWriter()

	c.lastRange = ra

	return res, nil
}

// Play sends a PLAY request.
// This can be called only after Setup().
func (c *Client) Play(ra *headers.Range) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.chPlay <- playReq{ra: ra, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.done:
		return nil, c.closeError
	}
}

func (c *Client) doRecord() (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	c.state = clientStateRecord
	c.startTransportRoutines()
	c.createWriter()

	res, err := c.do(&base.Request{
		Method: base.Record,
		URL:    c.baseURL,
	}, false)
	if err != nil {
		c.destroyWriter()
		c.stopTransportRoutines()
		c.state = clientStatePreRecord
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		c.destroyWriter()
		c.stopTransportRoutines()
		c.state = clientStatePreRecord
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.startWriter()

	return nil, nil
}

// Record sends a RECORD request.
// This can be called only after Announce() and Setup().
func (c *Client) Record() (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.chRecord <- recordReq{res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.done:
		return nil, c.closeError
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

	c.destroyWriter()

	res, err := c.do(&base.Request{
		Method: base.Pause,
		URL:    c.baseURL,
	}, false)
	if err != nil {
		c.createWriter()
		c.startWriter()
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		c.createWriter()
		c.startWriter()
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.stopTransportRoutines()

	switch c.state {
	case clientStatePlay:
		c.state = clientStatePrePlay
	case clientStateRecord:
		c.state = clientStatePreRecord
	}

	return res, nil
}

// Pause sends a PAUSE request.
// This can be called only after Play() or Record().
func (c *Client) Pause() (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.chPause <- pauseReq{res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.done:
		return nil, c.closeError
	}
}

// OnPacketRTPAny sets a callback that is called when a RTP packet is read from any setupped media.
func (c *Client) OnPacketRTPAny(cb OnPacketRTPAnyFunc) {
	for _, cm := range c.setuppedMedias {
		cmedia := cm.media
		for _, forma := range cm.media.Formats {
			c.OnPacketRTP(cm.media, forma, func(pkt *rtp.Packet) {
				cb(cmedia, forma, pkt)
			})
		}
	}
}

// OnPacketRTCPAny sets a callback that is called when a RTCP packet is read from any setupped media.
func (c *Client) OnPacketRTCPAny(cb OnPacketRTCPAnyFunc) {
	for _, cm := range c.setuppedMedias {
		cmedia := cm.media
		c.OnPacketRTCP(cm.media, func(pkt rtcp.Packet) {
			cb(cmedia, pkt)
		})
	}
}

// OnPacketRTP sets a callback that is called when a RTP packet is read.
func (c *Client) OnPacketRTP(medi *description.Media, forma format.Format, cb OnPacketRTPFunc) {
	cm := c.setuppedMedias[medi]
	ct := cm.formats[forma.PayloadType()]
	ct.onPacketRTP = cb
}

// OnPacketRTCP sets a callback that is called when a RTCP packet is read.
func (c *Client) OnPacketRTCP(medi *description.Media, cb OnPacketRTCPFunc) {
	cm := c.setuppedMedias[medi]
	cm.onPacketRTCP = cb
}

// WritePacketRTP writes a RTP packet to the server.
func (c *Client) WritePacketRTP(medi *description.Media, pkt *rtp.Packet) error {
	return c.WritePacketRTPWithNTP(medi, pkt, c.timeNow())
}

// WritePacketRTPWithNTP writes a RTP packet to the server.
// ntp is the absolute timestamp of the packet, and is sent with periodic RTCP sender reports.
func (c *Client) WritePacketRTPWithNTP(medi *description.Media, pkt *rtp.Packet, ntp time.Time) error {
	select {
	case <-c.done:
		return c.closeError
	default:
	}

	cm := c.setuppedMedias[medi]
	cf := cm.formats[pkt.PayloadType]
	return cf.writePacketRTP(pkt, ntp)
}

// WritePacketRTCP writes a RTCP packet to the server.
func (c *Client) WritePacketRTCP(medi *description.Media, pkt rtcp.Packet) error {
	select {
	case <-c.done:
		return c.closeError
	default:
	}

	cm := c.setuppedMedias[medi]
	return cm.writePacketRTCP(pkt)
}

// PacketPTS returns the PTS (presentation timestamp) of an incoming RTP packet.
// It is computed by decoding the packet timestamp and sychronizing it with other tracks.
func (c *Client) PacketPTS(medi *description.Media, pkt *rtp.Packet) (int64, bool) {
	cm := c.setuppedMedias[medi]
	ct := cm.formats[pkt.PayloadType]
	return c.timeDecoder.Decode(ct.format, pkt)
}

// PacketNTP returns the NTP (absolute timestamp) of an incoming RTP packet.
// The NTP is computed from RTCP sender reports.
func (c *Client) PacketNTP(medi *description.Media, pkt *rtp.Packet) (time.Time, bool) {
	cm := c.setuppedMedias[medi]
	ct := cm.formats[pkt.PayloadType]
	return ct.rtpReceiver.PacketNTP(pkt.Timestamp)
}

// Transport returns transport details.
func (c *Client) Transport() *ClientTransport {
	c.propsMutex.RLock()
	defer c.propsMutex.RUnlock()

	return &ClientTransport{
		Conn: ConnTransport{
			Tunnel: c.Tunnel,
		},
		Session: c.setuppedTransport,
	}
}

// Stats returns client statistics.
func (c *Client) Stats() *ClientStats {
	c.propsMutex.RLock()
	defer c.propsMutex.RUnlock()

	mediaStats := func() map[*description.Media]SessionStatsMedia { //nolint:dupl
		ret := make(map[*description.Media]SessionStatsMedia, len(c.setuppedMedias))

		for med, sm := range c.setuppedMedias {
			ret[med] = SessionStatsMedia{
				BytesReceived:       atomic.LoadUint64(sm.bytesReceived),
				BytesSent:           atomic.LoadUint64(sm.bytesSent),
				RTPPacketsInError:   atomic.LoadUint64(sm.rtpPacketsInError),
				RTCPPacketsReceived: atomic.LoadUint64(sm.rtcpPacketsReceived),
				RTCPPacketsSent:     atomic.LoadUint64(sm.rtcpPacketsSent),
				RTCPPacketsInError:  atomic.LoadUint64(sm.rtcpPacketsInError),
				Formats: func() map[format.Format]SessionStatsFormat {
					ret := make(map[format.Format]SessionStatsFormat, len(sm.formats))

					for _, fo := range sm.formats {
						recvStats := func() *rtpreceiver.Stats {
							if fo.rtpReceiver != nil {
								return fo.rtpReceiver.Stats()
							}
							return nil
						}()
						sentStats := func() *rtpsender.Stats {
							if fo.rtpSender != nil {
								return fo.rtpSender.Stats()
							}
							return nil
						}()

						ret[fo.format] = SessionStatsFormat{ //nolint:dupl
							RTPPacketsReceived: atomic.LoadUint64(fo.rtpPacketsReceived),
							RTPPacketsSent:     atomic.LoadUint64(fo.rtpPacketsSent),
							RTPPacketsLost:     atomic.LoadUint64(fo.rtpPacketsLost),
							LocalSSRC:          fo.localSSRC,
							RemoteSSRC: func() uint32 {
								if v, ok := fo.remoteSSRC(); ok {
									return v
								}
								return 0
							}(),
							RTPPacketsLastSequenceNumber: func() uint16 {
								if recvStats != nil {
									return recvStats.LastSequenceNumber
								}
								if sentStats != nil {
									return sentStats.LastSequenceNumber
								}
								return 0
							}(),
							RTPPacketsLastRTP: func() uint32 {
								if recvStats != nil {
									return recvStats.LastRTP
								}
								if sentStats != nil {
									return sentStats.LastRTP
								}
								return 0
							}(),
							RTPPacketsLastNTP: func() time.Time {
								if recvStats != nil {
									return recvStats.LastNTP
								}
								if sentStats != nil {
									return sentStats.LastNTP
								}
								return time.Time{}
							}(),
							RTPPacketsJitter: func() float64 {
								if recvStats != nil {
									return recvStats.Jitter
								}
								return 0
							}(),
						}
					}

					return ret
				}(),
			}
		}

		return ret
	}()

	return &ClientStats{
		Conn: ConnStats{
			BytesReceived: atomic.LoadUint64(c.bytesReceived),
			BytesSent:     atomic.LoadUint64(c.bytesSent),
		},
		Session: SessionStats{ //nolint:dupl
			BytesReceived: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					v += ms.BytesReceived
				}
				return v
			}(),
			BytesSent: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					v += ms.BytesSent
				}
				return v
			}(),
			RTPPacketsReceived: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					for _, f := range ms.Formats {
						v += f.RTPPacketsReceived
					}
				}
				return v
			}(),
			RTPPacketsSent: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					for _, f := range ms.Formats {
						v += f.RTPPacketsSent
					}
				}
				return v
			}(),
			RTPPacketsLost: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					for _, f := range ms.Formats {
						v += f.RTPPacketsLost
					}
				}
				return v
			}(),
			RTPPacketsInError: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					v += ms.RTPPacketsInError
				}
				return v
			}(),
			RTPPacketsJitter: func() float64 {
				v := float64(0)
				n := float64(0)
				for _, ms := range mediaStats {
					for _, f := range ms.Formats {
						v += f.RTPPacketsJitter
						n++
					}
				}
				if n != 0 {
					return v / n
				}
				return 0
			}(),
			RTCPPacketsReceived: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					v += ms.RTCPPacketsReceived
				}
				return v
			}(),
			RTCPPacketsSent: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					v += ms.RTCPPacketsSent
				}
				return v
			}(),
			RTCPPacketsInError: func() uint64 {
				v := uint64(0)
				for _, ms := range mediaStats {
					v += ms.RTCPPacketsInError
				}
				return v
			}(),
			Medias: mediaStats,
		},
	}
}
