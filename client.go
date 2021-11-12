/*
Package gortsplib is a RTSP 1.0 library for the Go programming language,
written for rtsp-simple-server.

Examples are available at https://github.com/aler9/gortsplib/tree/master/examples

*/
package gortsplib

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/auth"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
)

const (
	clientReadBufferSize     = 4096
	clientWriteBufferSize    = 4096
	clientCheckStreamPeriod  = 1 * time.Second
	clientUDPKeepalivePeriod = 30 * time.Second
)

func isErrNOUDPPacketsReceivedRecently(err error) bool {
	_, ok := err.(liberrors.ErrClientNoUDPPacketsRecently)
	return ok
}

func isAnyPort(p int) bool {
	return p == 0 || p == 1
}

type clientState int

const (
	clientStateInitial clientState = iota
	clientStatePrePlay
	clientStatePlay
	clientStatePreRecord
	clientStateRecord
)

type clientTrack struct {
	track           *Track
	udpRTPListener  *clientUDPListener
	udpRTCPListener *clientUDPListener
	tcpChannel      int
	rtcpReceiver    *rtcpreceiver.RTCPReceiver
	rtcpSender      *rtcpsender.RTCPSender
}

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
	url    *base.URL
	tracks Tracks
	res    chan clientRes
}

type setupReq struct {
	mode     headers.TransportMode
	baseURL  *base.URL
	track    *Track
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
	tracks  Tracks
	baseURL *base.URL
	res     *base.Response
	err     error
}

// Client is a RTSP client.
type Client struct {
	//
	// callbacks
	//
	// called before every request.
	OnRequest func(*base.Request)
	// called after every response.
	OnResponse func(*base.Response)
	// called before sending a PLAY request.
	OnPlay func(*Client)
	// called when a RTP packet arrives.
	OnPacketRTP func(*Client, int, []byte)
	// called when a RTCP packet arrives.
	OnPacketRTCP func(*Client, int, []byte)

	//
	// RTSP parameters
	//
	// timeout of read operations.
	// It defaults to 10 seconds.
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds.
	WriteTimeout time.Duration
	// a TLS configuration to connect to TLS (RTSPS) servers.
	// It defaults to &tls.Config{InsecureSkipVerify:true}
	TLSConfig *tls.Config
	// disable being redirected to other servers, that can happen during Describe().
	// It defaults to false.
	RedirectDisable bool
	// enable communication with servers which don't provide server ports.
	// this can be a security issue.
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
	// It defaults to 1.
	ReadBufferCount int
	// read buffer size.
	// This must be touched only when the server reports errors about buffer sizes.
	// It defaults to 2048.
	ReadBufferSize int

	//
	// system functions
	//
	// function used to initialize the TCP client.
	// It defaults to (&net.Dialer{}).DialContext.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
	// function used to initialize UDP listeners.
	// It defaults to net.ListenPacket.
	ListenPacket func(network, address string) (net.PacketConn, error)

	//
	// private
	//

	senderReportPeriod   time.Duration
	receiverReportPeriod time.Duration

	scheme            string
	host              string
	ctx               context.Context
	ctxCancel         func()
	state             clientState
	nconn             net.Conn
	br                *bufio.Reader
	bw                *bufio.Writer
	session           string
	sender            *auth.Sender
	cseq              int
	useGetParameter   bool
	streamBaseURL     *base.URL
	protocol          *Transport
	tracks            map[int]clientTrack
	tracksByChannel   map[int]int
	lastRange         *headers.Range
	backgroundRunning bool
	backgroundErr     error
	tcpFrameBuffer    *multibuffer.MultiBuffer // tcp
	tcpWriteMutex     sync.Mutex               // tcp
	writeMutex        sync.RWMutex             // write
	writeFrameAllowed bool                     // write

	// in
	options             chan optionsReq
	describe            chan describeReq
	announce            chan announceReq
	setup               chan setupReq
	play                chan playReq
	record              chan recordReq
	pause               chan pauseReq
	backgroundTerminate chan struct{}

	// out
	backgroundInnerDone chan error
	backgroundDone      chan struct{}
	done                chan struct{}
}

// Dial connects to a server.
func (c *Client) Dial(scheme string, host string) error {
	// callbacks
	if c.OnPacketRTP == nil {
		c.OnPacketRTP = func(c *Client, trackID int, payload []byte) {
		}
	}
	if c.OnPacketRTCP == nil {
		c.OnPacketRTCP = func(c *Client, trackID int, payload []byte) {
		}
	}

	// RTSP parameters
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 10 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}
	if c.InitialUDPReadTimeout == 0 {
		c.InitialUDPReadTimeout = 3 * time.Second
	}
	if c.ReadBufferCount == 0 {
		c.ReadBufferCount = 1
	}
	if c.ReadBufferSize == 0 {
		c.ReadBufferSize = 2048
	}

	// system functions
	if c.DialContext == nil {
		c.DialContext = (&net.Dialer{}).DialContext
	}
	if c.ListenPacket == nil {
		c.ListenPacket = net.ListenPacket
	}

	// private
	if c.senderReportPeriod == 0 {
		c.senderReportPeriod = 10 * time.Second
	}
	if c.receiverReportPeriod == 0 {
		c.receiverReportPeriod = 10 * time.Second
	}

	ctx, ctxCancel := context.WithCancel(context.Background())

	c.scheme = scheme
	c.host = host
	c.ctx = ctx
	c.ctxCancel = ctxCancel
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

// DialRead connects to the address and starts reading all tracks.
func (c *Client) DialRead(address string) error {
	u, err := base.ParseURL(address)
	if err != nil {
		return err
	}

	err = c.Dial(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	_, err = c.Options(u)
	if err != nil {
		c.Close()
		return err
	}

	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		c.Close()
		return err
	}

	for _, track := range tracks {
		_, err := c.Setup(headers.TransportModePlay, baseURL, track, 0, 0)
		if err != nil {
			c.Close()
			return err
		}
	}

	_, err = c.Play(nil)
	if err != nil {
		c.Close()
		return err
	}

	return nil
}

// DialPublish connects to the address and starts publishing the tracks.
func (c *Client) DialPublish(address string, tracks Tracks) error {
	u, err := base.ParseURL(address)
	if err != nil {
		return err
	}

	err = c.Dial(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	_, err = c.Options(u)
	if err != nil {
		c.Close()
		return err
	}

	_, err = c.Announce(u, tracks)
	if err != nil {
		c.Close()
		return err
	}

	for _, track := range tracks {
		_, err := c.Setup(headers.TransportModeRecord, u, track, 0, 0)
		if err != nil {
			c.Close()
			return err
		}
	}

	_, err = c.Record()
	if err != nil {
		c.Close()
		return err
	}

	return nil
}

// Close closes all the client resources and waits for them to exit.
func (c *Client) Close() error {
	c.ctxCancel()
	<-c.done
	return nil
}

// Tracks returns all the tracks that the client is reading or publishing.
func (c *Client) Tracks() Tracks {
	ids := make([]int, len(c.tracks))
	pos := 0
	for id := range c.tracks {
		ids[pos] = id
		pos++
	}
	sort.Slice(ids, func(a, b int) bool {
		return ids[a] < ids[b]
	})

	var ret Tracks
	for _, id := range ids {
		ret = append(ret, c.tracks[id].track)
	}
	return ret
}

func (c *Client) run() {
	defer close(c.done)

outer:
	for {
		select {
		case req := <-c.options:
			res, err := c.doOptions(req.url)
			req.res <- clientRes{res: res, err: err}

		case req := <-c.describe:
			tracks, baseURL, res, err := c.doDescribe(req.url)
			req.res <- clientRes{tracks: tracks, baseURL: baseURL, res: res, err: err}

		case req := <-c.announce:
			res, err := c.doAnnounce(req.url, req.tracks)
			req.res <- clientRes{res: res, err: err}

		case req := <-c.setup:
			res, err := c.doSetup(req.mode, req.baseURL, req.track, req.rtpPort, req.rtcpPort)
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

		case err := <-c.backgroundInnerDone:
			c.backgroundRunning = false
			err = c.switchProtocolIfTimeout(err)
			if err != nil {
				c.backgroundErr = err
				close(c.backgroundDone)

				c.writeMutex.Lock()
				c.writeFrameAllowed = false
				c.writeMutex.Unlock()
			}

		case <-c.ctx.Done():
			break outer
		}
	}

	c.ctxCancel()

	c.doClose(false)
}

func (c *Client) doClose(isSwitchingProtocol bool) {
	if c.backgroundRunning {
		c.backgroundClose(isSwitchingProtocol)
	}

	if c.state == clientStatePlay || c.state == clientStateRecord {
		c.do(&base.Request{
			Method: base.Teardown,
			URL:    c.streamBaseURL,
		}, true)
	}

	for _, track := range c.tracks {
		if track.udpRTPListener != nil {
			track.udpRTPListener.close()
			track.udpRTCPListener.close()
		}
	}

	if c.nconn != nil {
		c.nconn.Close()
		c.nconn = nil
	}
}

func (c *Client) reset(isSwitchingProtocol bool) {
	c.doClose(isSwitchingProtocol)

	c.state = clientStateInitial
	c.session = ""
	c.sender = nil
	c.cseq = 0
	c.useGetParameter = false
	c.streamBaseURL = nil
	c.protocol = nil
	c.tracks = nil
	c.tracksByChannel = nil
	c.tcpFrameBuffer = nil
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

func (c *Client) switchProtocolIfTimeout(err error) error {
	if *c.protocol != TransportUDP ||
		c.state != clientStatePlay ||
		!isErrNOUDPPacketsReceivedRecently(err) ||
		c.Transport != nil {
		return err
	}

	prevBaseURL := c.streamBaseURL
	oldUseGetParameter := c.useGetParameter
	prevTracks := c.tracks

	c.reset(true)

	v := TransportTCP
	c.protocol = &v
	c.useGetParameter = oldUseGetParameter
	c.scheme = prevBaseURL.Scheme
	c.host = prevBaseURL.Host

	for _, track := range prevTracks {
		_, err := c.doSetup(headers.TransportModePlay, prevBaseURL, track.track, 0, 0)
		if err != nil {
			return err
		}
	}

	_, err = c.doPlay(c.lastRange, true)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) backgroundStart(isSwitchingProtocol bool) {
	c.writeMutex.Lock()
	c.writeFrameAllowed = true
	c.writeMutex.Unlock()

	c.backgroundRunning = true
	c.backgroundTerminate = make(chan struct{})
	c.backgroundInnerDone = make(chan error)

	if !isSwitchingProtocol {
		c.backgroundDone = make(chan struct{})
	}

	go c.runBackground()
}

func (c *Client) backgroundClose(isSwitchingProtocol bool) {
	close(c.backgroundTerminate)
	err := <-c.backgroundInnerDone
	c.backgroundRunning = false

	if !isSwitchingProtocol {
		c.backgroundErr = err
		close(c.backgroundDone)
	}

	c.writeMutex.Lock()
	c.writeFrameAllowed = false
	c.writeMutex.Unlock()
}

func (c *Client) runBackground() {
	c.backgroundInnerDone <- func() error {
		if c.state == clientStatePlay {
			if *c.protocol == TransportUDP || *c.protocol == TransportUDPMulticast {
				return c.runBackgroundPlayUDP()
			}
			return c.runBackgroundPlayTCP()
		}

		if *c.protocol == TransportUDP {
			return c.runBackgroundRecordUDP()
		}
		return c.runBackgroundRecordTCP()
	}()
}

func (c *Client) runBackgroundPlayUDP() error {
	for _, cct := range c.tracks {
		cct.udpRTPListener.start()
		cct.udpRTCPListener.start()
	}

	defer func() {
		for _, cct := range c.tracks {
			cct.udpRTPListener.stop()
			cct.udpRTCPListener.stop()
		}
	}()

	// disable deadline
	c.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			var res base.Response
			err := res.Read(c.br)
			if err != nil {
				readerDone <- err
				return
			}
		}
	}()

	reportTicker := time.NewTicker(c.receiverReportPeriod)
	defer reportTicker.Stop()

	keepaliveTicker := time.NewTicker(clientUDPKeepalivePeriod)
	defer keepaliveTicker.Stop()

	checkStreamInitial := true
	checkStreamTicker := time.NewTicker(c.InitialUDPReadTimeout)
	defer func() {
		checkStreamTicker.Stop()
	}()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range c.tracks {
				rr := cct.rtcpReceiver.Report(now)
				c.WritePacketRTCP(trackID, rr)
			}

		case <-keepaliveTicker.C:
			_, err := c.do(&base.Request{
				Method: func() base.Method {
					// the vlc integrated rtsp server requires GET_PARAMETER
					if c.useGetParameter {
						return base.GetParameter
					}
					return base.Options
				}(),
				// use the stream base URL, otherwise some cameras do not reply
				URL: c.streamBaseURL,
			}, true)
			if err != nil {
				c.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return err
			}

		case <-checkStreamTicker.C:
			if checkStreamInitial {
				// check that at least one packet has been received
				inTimeout := func() bool {
					for _, cct := range c.tracks {
						lft := atomic.LoadInt64(cct.udpRTPListener.lastFrameTime)
						if lft != 0 {
							return false
						}

						lft = atomic.LoadInt64(cct.udpRTCPListener.lastFrameTime)
						if lft != 0 {
							return false
						}
					}
					return true
				}()
				if inTimeout {
					c.nconn.SetReadDeadline(time.Now())
					<-readerDone
					return liberrors.ErrClientNoUDPPacketsRecently{}
				}

				checkStreamInitial = false
				checkStreamTicker.Stop()
				checkStreamTicker = time.NewTicker(clientCheckStreamPeriod)
			} else {
				inTimeout := func() bool {
					now := time.Now()
					for _, cct := range c.tracks {
						lft := time.Unix(atomic.LoadInt64(cct.udpRTPListener.lastFrameTime), 0)
						if now.Sub(lft) < c.ReadTimeout {
							return false
						}

						lft = time.Unix(atomic.LoadInt64(cct.udpRTCPListener.lastFrameTime), 0)
						if now.Sub(lft) < c.ReadTimeout {
							return false
						}
					}
					return true
				}()
				if inTimeout {
					c.nconn.SetReadDeadline(time.Now())
					<-readerDone
					return liberrors.ErrClientUDPTimeout{}
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (c *Client) runBackgroundPlayTCP() error {
	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we disable the deadline and perform check with a ticker.
	c.nconn.SetReadDeadline(time.Time{})

	lastFrameTime := time.Now().Unix()

	readerDone := make(chan error)
	go func() {
		for {
			frame := base.InterleavedFrame{
				Payload: c.tcpFrameBuffer.Next(),
			}
			err := frame.Read(c.br)
			if err != nil {
				readerDone <- err
				return
			}

			channel := frame.Channel
			isRTP := true
			if (channel % 2) != 0 {
				channel--
				isRTP = false
			}

			trackID, ok := c.tracksByChannel[channel]
			if !ok {
				continue
			}

			now := time.Now()
			atomic.StoreInt64(&lastFrameTime, now.Unix())

			if isRTP {
				c.tracks[trackID].rtcpReceiver.ProcessPacketRTP(now, frame.Payload)
				c.OnPacketRTP(c, trackID, frame.Payload)
			} else {
				c.tracks[trackID].rtcpReceiver.ProcessPacketRTCP(now, frame.Payload)
				c.OnPacketRTCP(c, trackID, frame.Payload)
			}
		}
	}()

	reportTicker := time.NewTicker(c.receiverReportPeriod)
	defer reportTicker.Stop()

	checkStreamTicker := time.NewTicker(clientCheckStreamPeriod)
	defer checkStreamTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range c.tracks {
				rr := cct.rtcpReceiver.Report(now)
				c.WritePacketRTCP(trackID, rr)
			}

		case <-checkStreamTicker.C:
			inTimeout := func() bool {
				now := time.Now()
				lft := time.Unix(atomic.LoadInt64(&lastFrameTime), 0)
				return now.Sub(lft) >= c.ReadTimeout
			}()
			if inTimeout {
				c.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return liberrors.ErrClientTCPTimeout{}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (c *Client) runBackgroundRecordUDP() error {
	for _, cct := range c.tracks {
		cct.udpRTPListener.start()
		cct.udpRTCPListener.start()
	}

	defer func() {
		for _, cct := range c.tracks {
			cct.udpRTPListener.stop()
			cct.udpRTCPListener.stop()
		}
	}()

	// disable deadline
	c.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			var res base.Response
			err := res.Read(c.br)
			if err != nil {
				readerDone <- err
				return
			}
		}
	}()

	reportTicker := time.NewTicker(c.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range c.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					c.WritePacketRTCP(trackID, sr)
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (c *Client) runBackgroundRecordTCP() error {
	// disable deadline
	c.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			frame := base.InterleavedFrame{
				Payload: c.tcpFrameBuffer.Next(),
			}
			err := frame.Read(c.br)
			if err != nil {
				readerDone <- err
				return
			}

			channel := frame.Channel
			isRTP := true
			if (channel % 2) != 0 {
				channel--
				isRTP = false
			}

			trackID, ok := c.tracksByChannel[channel]
			if !ok {
				continue
			}

			if !isRTP {
				c.OnPacketRTCP(c, trackID, frame.Payload)
			}
		}
	}()

	reportTicker := time.NewTicker(c.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range c.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					c.WritePacketRTCP(trackID, sr)
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (c *Client) connOpen() error {
	if c.scheme != "rtsp" && c.scheme != "rtsps" {
		return fmt.Errorf("unsupported scheme '%s'", c.scheme)
	}

	if c.scheme == "rtsps" && c.Transport != nil && *c.Transport != TransportTCP {
		return fmt.Errorf("RTSPS can be used only with TCP")
	}

	if !strings.Contains(c.host, ":") {
		c.host += ":554"
	}

	ctx, cancel := context.WithTimeout(c.ctx, c.ReadTimeout)
	defer cancel()

	nconn, err := c.DialContext(ctx, "tcp", c.host)
	if err != nil {
		return err
	}

	conn := func() net.Conn {
		if c.scheme == "rtsps" {
			return tls.Client(nconn, c.TLSConfig)
		}
		return nconn
	}()

	c.nconn = nconn
	c.br = bufio.NewReaderSize(conn, clientReadBufferSize)
	c.bw = bufio.NewWriterSize(conn, clientWriteBufferSize)
	return nil
}

func (c *Client) do(req *base.Request, skipResponse bool) (*base.Response, error) {
	if c.nconn == nil {
		err := c.connOpen()
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

	if c.sender != nil {
		c.sender.AddAuthorization(req)
	}

	c.cseq++
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(c.cseq), 10)}

	req.Header["User-Agent"] = base.HeaderValue{"gortsplib"}

	if c.OnRequest != nil {
		c.OnRequest(req)
	}

	var res base.Response

	err := func() error {
		// the only two do() with skipResponses are
		// - TEARDOWN -> ctx is already canceled, so this can't be used
		// - keepalives -> if ctx is canceled during a keepalive,
		//   it's better not to stop the request, but wait until teardown
		if !skipResponse {
			ctxHandlerDone := make(chan struct{})
			defer func() { <-ctxHandlerDone }()

			ctxHandlerTerminate := make(chan struct{})
			defer close(ctxHandlerTerminate)

			go func() {
				defer close(ctxHandlerDone)
				select {
				case <-c.ctx.Done():
					c.nconn.Close()
				case <-ctxHandlerTerminate:
				}
			}()
		}

		c.nconn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
		err := req.Write(c.bw)
		if err != nil {
			return err
		}

		if skipResponse {
			return nil
		}

		c.nconn.SetReadDeadline(time.Now().Add(c.ReadTimeout))

		if c.tcpFrameBuffer != nil {
			// read the response and ignore interleaved frames in between;
			// interleaved frames are sent in two scenarios:
			// * when the server is v4lrtspserver, before the PLAY response
			// * when the stream is already playing
			err = res.ReadIgnoreFrames(c.br, c.tcpFrameBuffer.Next())
			if err != nil {
				return err
			}
		} else {
			err = res.Read(c.br)
			if err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	if c.OnResponse != nil {
		c.OnResponse(&res)
	}

	// get session from response
	if v, ok := res.Header["Session"]; ok {
		var sx headers.Session
		err := sx.Read(v)
		if err != nil {
			return nil, liberrors.ErrClientSessionHeaderInvalid{Err: err}
		}
		c.session = sx.Session
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

		return c.do(req, false)
	}

	return &res, nil
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

	res, err := c.do(&base.Request{
		Method: base.Options,
		URL:    u,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		// since this method is not implemented by every RTSP server,
		// return only if status code is not 404
		if res.StatusCode == base.StatusNotFound {
			return res, nil
		}
		return res, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

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
func (c *Client) Options(u *base.URL) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.options <- optionsReq{url: u, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doDescribe(u *base.URL) (Tracks, *base.URL, *base.Response, error) {
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
	}, false)
	if err != nil {
		return nil, nil, nil, err
	}

	if res.StatusCode != base.StatusOK {
		// redirect
		if !c.RedirectDisable &&
			res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {
			c.reset(false)

			u, err := base.ParseURL(res.Header["Location"][0])
			if err != nil {
				return nil, nil, nil, err
			}

			c.scheme = u.Scheme
			c.host = u.Host

			_, err = c.doOptions(u)
			if err != nil {
				return nil, nil, nil, err
			}

			return c.doDescribe(u)
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

	baseURL, err := func() (*base.URL, error) {
		// use Content-Base
		if cb, ok := res.Header["Content-Base"]; ok {
			if len(cb) != 1 {
				return nil, fmt.Errorf("invalid Content-Base: '%v'", cb)
			}

			ret, err := base.ParseURL(cb[0])
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Base: '%v'", cb)
			}

			// add credentials from URL of request
			ret.User = u.User

			return ret, nil
		}

		// if not provided, use URL of request
		return u, nil
	}()
	if err != nil {
		return nil, nil, nil, err
	}

	tracks, err := ReadTracks(res.Body)
	if err != nil {
		return nil, nil, nil, err
	}

	return tracks, baseURL, res, nil
}

// Describe writes a DESCRIBE request and reads a Response.
func (c *Client) Describe(u *base.URL) (Tracks, *base.URL, *base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.describe <- describeReq{url: u, res: cres}:
		res := <-cres
		return res.tracks, res.baseURL, res.res, res.err

	case <-c.ctx.Done():
		return nil, nil, nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doAnnounce(u *base.URL, tracks Tracks) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial: {},
	})
	if err != nil {
		return nil, err
	}

	// in case of ANNOUNCE, the base URL doesn't have a trailing slash.
	// (tested with ffmpeg and gstreamer)
	baseURL := u.Clone()

	for i, t := range tracks {
		if !t.hasControlAttribute() {
			t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
				Key:   "control",
				Value: "trackID=" + strconv.FormatInt(int64(i), 10),
			})
		}
	}

	res, err := c.do(&base.Request{
		Method: base.Announce,
		URL:    u,
		Header: base.Header{
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.streamBaseURL = baseURL
	c.state = clientStatePreRecord

	return res, nil
}

// Announce writes an ANNOUNCE request and reads a Response.
func (c *Client) Announce(u *base.URL, tracks Tracks) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.announce <- announceReq{url: u, tracks: tracks, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-c.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (c *Client) doSetup(
	mode headers.TransportMode,
	baseURL *base.URL,
	track *Track,
	rtpPort int,
	rtcpPort int) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStateInitial:   {},
		clientStatePrePlay:   {},
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	if (mode == headers.TransportModeRecord && c.state != clientStatePreRecord) ||
		(mode == headers.TransportModePlay && c.state != clientStatePrePlay &&
			c.state != clientStateInitial) {
		return nil, liberrors.ErrClientCannotReadPublishAtSameTime{}
	}

	if c.streamBaseURL != nil && *baseURL != *c.streamBaseURL {
		return nil, liberrors.ErrClientCannotSetupTracksDifferentURLs{}
	}

	var rtpListener *clientUDPListener
	var rtcpListener *clientUDPListener

	// always use TCP if encrypted
	if c.scheme == "rtsps" {
		v := TransportTCP
		c.protocol = &v
	}

	proto := func() Transport {
		// protocol set by previous Setup() or switchProtocolIfTimeout()
		if c.protocol != nil {
			return *c.protocol
		}

		// protocol set by conf
		if c.Transport != nil {
			return *c.Transport
		}

		// try UDP
		return TransportUDP
	}()

	th := headers.Transport{
		Mode: &mode,
	}

	trackID := len(c.tracks)

	switch proto {
	case TransportUDP:
		if (rtpPort == 0 && rtcpPort != 0) ||
			(rtpPort != 0 && rtcpPort == 0) {
			return nil, liberrors.ErrClientUDPPortsZero{}
		}

		if rtpPort != 0 && rtcpPort != (rtpPort+1) {
			return nil, liberrors.ErrClientUDPPortsNotConsecutive{}
		}

		var err error
		if rtpPort != 0 {
			rtpListener, err = newClientUDPListener(c, false, ":"+strconv.FormatInt(int64(rtpPort), 10))
			if err != nil {
				return nil, err
			}

			rtcpListener, err = newClientUDPListener(c, false, ":"+strconv.FormatInt(int64(rtcpPort), 10))
			if err != nil {
				rtpListener.close()
				return nil, err
			}
		} else {
			rtpListener, rtcpListener = newClientUDPListenerPair(c)
		}

		v1 := headers.TransportDeliveryUnicast
		th.Delivery = &v1
		th.Protocol = headers.TransportProtocolUDP
		th.ClientPorts = &[2]int{
			rtpListener.port(),
			rtcpListener.port(),
		}

	case TransportUDPMulticast:
		v1 := headers.TransportDeliveryMulticast
		th.Delivery = &v1
		th.Protocol = headers.TransportProtocolUDP

	case TransportTCP:
		v1 := headers.TransportDeliveryUnicast
		th.Delivery = &v1
		th.Protocol = headers.TransportProtocolTCP
		th.InterleavedIDs = &[2]int{(trackID * 2), (trackID * 2) + 1}
	}

	trackURL, err := track.URL(baseURL)
	if err != nil {
		if proto == TransportUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, err
	}

	res, err := c.do(&base.Request{
		Method: base.Setup,
		URL:    trackURL,
		Header: base.Header{
			"Transport": th.Write(),
		},
	}, false)
	if err != nil {
		if proto == TransportUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		if proto == TransportUDP {
			rtpListener.close()
			rtcpListener.close()
		}

		// switch protocol automatically
		if res.StatusCode == base.StatusUnsupportedTransport &&
			c.protocol == nil &&
			c.Transport == nil {
			v := TransportTCP
			c.protocol = &v

			return c.doSetup(mode, baseURL, track, 0, 0)
		}

		return res, liberrors.ErrClientBadStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	var thRes headers.Transport
	err = thRes.Read(res.Header["Transport"])
	if err != nil {
		if proto == TransportUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, liberrors.ErrClientTransportHeaderInvalid{Err: err}
	}

	switch proto {
	case TransportUDP:
		if thRes.Delivery != nil && *thRes.Delivery != headers.TransportDeliveryUnicast {
			return nil, liberrors.ErrClientTransportHeaderInvalidDelivery{}
		}

		if !c.AnyPortEnable {
			if thRes.ServerPorts == nil || isAnyPort(thRes.ServerPorts[0]) || isAnyPort(thRes.ServerPorts[1]) {
				rtpListener.close()
				rtcpListener.close()
				return nil, liberrors.ErrClientServerPortsNotProvided{}
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

		rtpListener, err = newClientUDPListener(c, true,
			thRes.Destination.String()+":"+strconv.FormatInt(int64(thRes.Ports[0]), 10))
		if err != nil {
			return nil, err
		}

		rtcpListener, err = newClientUDPListener(c, true,
			thRes.Destination.String()+":"+strconv.FormatInt(int64(thRes.Ports[1]), 10))
		if err != nil {
			rtpListener.close()
			return nil, err
		}

	case TransportTCP:
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

		if _, ok := c.tracksByChannel[thRes.InterleavedIDs[0]]; ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrClientTransportHeaderInterleavedIDsAlreadyUsed{}
		}
	}

	clockRate, _ := track.ClockRate()
	cct := clientTrack{
		track: track,
	}

	if mode == headers.TransportModePlay {
		c.state = clientStatePrePlay
		cct.rtcpReceiver = rtcpreceiver.New(nil, clockRate)
	} else {
		c.state = clientStatePreRecord
		cct.rtcpSender = rtcpsender.New(clockRate)
	}

	c.streamBaseURL = baseURL
	c.protocol = &proto

	switch proto {
	case TransportUDP:
		rtpListener.remoteReadIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteWriteIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtpListener.remotePort = thRes.ServerPorts[0]
		}
		rtpListener.trackID = trackID
		rtpListener.streamType = StreamTypeRTP
		cct.udpRTPListener = rtpListener

		rtcpListener.remoteReadIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteWriteIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtcpListener.remotePort = thRes.ServerPorts[1]
		}
		rtcpListener.trackID = trackID
		rtcpListener.streamType = StreamTypeRTCP
		cct.udpRTCPListener = rtcpListener

	case TransportUDPMulticast:
		rtpListener.remoteReadIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteWriteIP = *thRes.Destination
		rtpListener.remoteZone = ""
		rtpListener.remotePort = thRes.Ports[0]
		rtpListener.trackID = trackID
		rtpListener.streamType = StreamTypeRTP
		cct.udpRTPListener = rtpListener

		rtcpListener.remoteReadIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteWriteIP = *thRes.Destination
		rtcpListener.remoteZone = ""
		rtcpListener.remotePort = thRes.Ports[1]
		rtcpListener.trackID = trackID
		rtcpListener.streamType = StreamTypeRTCP
		cct.udpRTCPListener = rtcpListener

	case TransportTCP:
		if c.tcpFrameBuffer == nil {
			c.tcpFrameBuffer = multibuffer.New(uint64(c.ReadBufferCount), uint64(c.ReadBufferSize))
		}

		if c.tracksByChannel == nil {
			c.tracksByChannel = make(map[int]int)
		}

		c.tracksByChannel[thRes.InterleavedIDs[0]] = trackID

		cct.tcpChannel = thRes.InterleavedIDs[0]
	}

	if c.tracks == nil {
		c.tracks = make(map[int]clientTrack)
	}

	c.tracks[trackID] = cct

	return res, nil
}

// Setup writes a SETUP request and reads a Response.
// rtpPort and rtcpPort are used only if protocol is UDP.
// if rtpPort and rtcpPort are zero, they are chosen automatically.
func (c *Client) Setup(
	mode headers.TransportMode,
	baseURL *base.URL,
	track *Track,
	rtpPort int,
	rtcpPort int) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.setup <- setupReq{
		mode:     mode,
		baseURL:  baseURL,
		track:    track,
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

func (c *Client) doPlay(ra *headers.Range, isSwitchingProtocol bool) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	// open the firewall by sending packets to the counterpart.
	// do this before sending the PLAY request.
	if *c.protocol == TransportUDP {
		for _, cct := range c.tracks {
			cct.udpRTPListener.write(
				[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

			cct.udpRTCPListener.write(
				[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
		}
	}

	if c.OnPlay != nil {
		c.OnPlay(c)
	}

	header := make(base.Header)

	// Range is mandatory in Parrot Streaming Server
	if ra == nil {
		ra = &headers.Range{
			Value: &headers.RangeNPT{
				Start: headers.RangeNPTTime(0),
			},
		}
	}
	header["Range"] = ra.Write()

	res, err := c.do(&base.Request{
		Method: base.Play,
		URL:    c.streamBaseURL,
		Header: header,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.state = clientStatePlay
	c.lastRange = ra

	c.backgroundStart(isSwitchingProtocol)

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
		URL:    c.streamBaseURL,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.state = clientStateRecord

	c.backgroundStart(false)

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

	c.backgroundClose(false)

	res, err := c.do(&base.Request{
		Method: base.Pause,
		URL:    c.streamBaseURL,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return res, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	switch c.state {
	case clientStatePlay:
		c.state = clientStatePrePlay
	case clientStateRecord:
		c.state = clientStatePreRecord
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

// ReadFrames starts reading frames.
func (c *Client) ReadFrames() error {
	<-c.backgroundDone
	return c.backgroundErr
}

// WritePacketRTP writes a RTP packet.
func (c *Client) WritePacketRTP(trackID int, payload []byte) error {
	now := time.Now()

	c.writeMutex.RLock()
	defer c.writeMutex.RUnlock()

	if !c.writeFrameAllowed {
		return c.backgroundErr
	}

	if c.tracks[trackID].rtcpSender != nil {
		c.tracks[trackID].rtcpSender.ProcessPacketRTP(now, payload)
	}

	switch *c.protocol {
	case TransportUDP, TransportUDPMulticast:
		return c.tracks[trackID].udpRTPListener.write(payload)

	default: // TCP
		channel := c.tracks[trackID].tcpChannel

		c.tcpWriteMutex.Lock()
		defer c.tcpWriteMutex.Unlock()

		c.nconn.SetWriteDeadline(now.Add(c.WriteTimeout))
		return base.InterleavedFrame{
			Channel: channel,
			Payload: payload,
		}.Write(c.bw)
	}
}

// WritePacketRTCP writes a RTCP packet.
func (c *Client) WritePacketRTCP(trackID int, payload []byte) error {
	now := time.Now()

	c.writeMutex.RLock()
	defer c.writeMutex.RUnlock()

	if !c.writeFrameAllowed {
		return c.backgroundErr
	}

	if c.tracks[trackID].rtcpSender != nil {
		c.tracks[trackID].rtcpSender.ProcessPacketRTCP(now, payload)
	}

	switch *c.protocol {
	case TransportUDP, TransportUDPMulticast:
		return c.tracks[trackID].udpRTCPListener.write(payload)

	default: // TCP
		channel := c.tracks[trackID].tcpChannel
		channel++

		c.tcpWriteMutex.Lock()
		defer c.tcpWriteMutex.Unlock()

		c.nconn.SetWriteDeadline(now.Add(c.WriteTimeout))
		return base.InterleavedFrame{
			Channel: channel,
			Payload: payload,
		}.Write(c.bw)
	}
}
