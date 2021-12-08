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
	clientReadBufferSize          = 4096
	clientWriteBufferSize         = 4096
	clientUDPKernelReadBufferSize = 0x80000 // same size as gstreamer's rtspsrc
)

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
	tcpRTPFrame     *base.InterleavedFrame
	tcpRTCPFrame    *base.InterleavedFrame
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
	forPlay  bool
	track    *Track
	baseURL  *base.URL
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
	// called when a RTP packet arrives.
	OnPacketRTP func(int, []byte)
	// called when a RTCP packet arrives.
	OnPacketRTCP func(int, []byte)

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

	udpSenderReportPeriod   time.Duration
	udpReceiverReportPeriod time.Duration
	checkStreamPeriod       time.Duration
	keepalivePeriod         time.Duration

	scheme             string
	host               string
	ctx                context.Context
	ctxCancel          func()
	state              clientState
	nconn              net.Conn
	br                 *bufio.Reader
	bw                 *bufio.Writer
	session            string
	sender             *auth.Sender
	cseq               int
	useGetParameter    bool
	streamBaseURL      *base.URL
	protocol           *Transport
	tracks             map[int]clientTrack
	tracksByChannel    map[int]int
	lastRange          *headers.Range
	tcpReadBuffer      *multibuffer.MultiBuffer
	tcpWriteMutex      sync.Mutex
	writeMutex         sync.RWMutex // publish
	writeFrameAllowed  bool         // publish
	udpReportTimer     *time.Timer
	checkStreamTimer   *time.Timer
	checkStreamInitial bool
	tcpLastFrameTime   int64
	keepaliveTimer     *time.Timer
	closeError         error

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
	// callbacks
	if c.OnPacketRTP == nil {
		c.OnPacketRTP = func(trackID int, payload []byte) {
		}
	}
	if c.OnPacketRTCP == nil {
		c.OnPacketRTCP = func(trackID int, payload []byte) {
		}
	}

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
	if c.udpSenderReportPeriod == 0 {
		c.udpSenderReportPeriod = 10 * time.Second
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
	c.udpReportTimer = emptyTimer()
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

// StartReading connects to the address and starts reading all tracks.
func (c *Client) StartReading(address string) error {
	u, err := base.ParseURL(address)
	if err != nil {
		return err
	}

	err = c.Start(u.Scheme, u.Host)
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

	return c.SetupAndPlay(tracks, baseURL)
}

// StartReadingAndWait connects to the address, starts reading all tracks and waits
// until a read error.
func (c *Client) StartReadingAndWait(address string) error {
	err := c.StartReading(address)
	if err != nil {
		return err
	}

	return c.Wait()
}

// StartPublishing connects to the address and starts publishing the tracks.
func (c *Client) StartPublishing(address string, tracks Tracks) error {
	u, err := base.ParseURL(address)
	if err != nil {
		return err
	}

	err = c.Start(u.Scheme, u.Host)
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
		_, err := c.Setup(false, track, u, 0, 0)
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

	c.closeError = func() error {
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
				res, err := c.doSetup(req.forPlay, req.track, req.baseURL, req.rtpPort, req.rtcpPort)
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

			case <-c.udpReportTimer.C:
				if c.state == clientStatePlay {
					now := time.Now()
					for trackID, cct := range c.tracks {
						rr := cct.rtcpReceiver.Report(now)
						if rr != nil {
							c.WritePacketRTCP(trackID, rr)
						}
					}

					c.udpReportTimer = time.NewTimer(c.udpReceiverReportPeriod)
				} else { // Record
					now := time.Now()
					for trackID, cct := range c.tracks {
						sr := cct.rtcpSender.Report(now)
						if sr != nil {
							c.WritePacketRTCP(trackID, sr)
						}
					}

					c.udpReportTimer = time.NewTimer(c.udpSenderReportPeriod)
				}

			case <-c.checkStreamTimer.C:
				if *c.protocol == TransportUDP ||
					*c.protocol == TransportUDPMulticast {
					if c.checkStreamInitial {
						c.checkStreamInitial = false

						// check that at least one packet has been received
						inTimeout := func() bool {
							for _, cct := range c.tracks {
								lft := atomic.LoadInt64(cct.udpRTPListener.lastPacketTime)
								if lft != 0 {
									return false
								}

								lft = atomic.LoadInt64(cct.udpRTCPListener.lastPacketTime)
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
							for _, cct := range c.tracks {
								lft := time.Unix(atomic.LoadInt64(cct.udpRTPListener.lastPacketTime), 0)
								if now.Sub(lft) < c.ReadTimeout {
									return false
								}

								lft = time.Unix(atomic.LoadInt64(cct.udpRTCPListener.lastPacketTime), 0)
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
						lft := time.Unix(atomic.LoadInt64(&c.tcpLastFrameTime), 0)
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
					URL: c.streamBaseURL,
				}, true)
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
	}()

	c.ctxCancel()

	c.doClose(true)
}

func (c *Client) doClose(isClosing bool) {
	if c.state == clientStatePlay || c.state == clientStateRecord {
		if *c.protocol == TransportUDP || *c.protocol == TransportUDPMulticast {
			// stop UDP listeners
			for _, cct := range c.tracks {
				cct.udpRTPListener.stop()
				cct.udpRTCPListener.stop()
			}
		}

		c.playRecordStop(isClosing)

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
		c.connCloserStop()
		c.nconn.Close()
		c.nconn = nil
	}
}

func (c *Client) reset() {
	c.doClose(false)

	c.state = clientStateInitial
	c.session = ""
	c.sender = nil
	c.cseq = 0
	c.useGetParameter = false
	c.streamBaseURL = nil
	c.protocol = nil
	c.tracks = nil
	c.tracksByChannel = nil
	c.tcpReadBuffer = nil
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
	prevBaseURL := c.streamBaseURL
	oldUseGetParameter := c.useGetParameter
	prevTracks := c.tracks

	c.reset()

	v := TransportTCP
	c.protocol = &v
	c.useGetParameter = oldUseGetParameter
	c.scheme = prevBaseURL.Scheme
	c.host = prevBaseURL.Host

	for _, track := range prevTracks {
		_, err := c.doSetup(true, track.track, prevBaseURL, 0, 0)
		if err != nil {
			return err
		}
	}

	_, err := c.doPlay(c.lastRange, true)
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) playRecordStart() {
	// stop connCloser
	c.connCloserStop()

	// allow writing
	c.writeMutex.Lock()
	c.writeFrameAllowed = true
	c.writeMutex.Unlock()

	// start timers
	if c.state == clientStatePlay {
		c.keepaliveTimer = time.NewTimer(c.keepalivePeriod)

		switch *c.protocol {
		case TransportUDP:
			c.udpReportTimer = time.NewTimer(c.udpReceiverReportPeriod)
			c.checkStreamTimer = time.NewTimer(c.InitialUDPReadTimeout)
			c.checkStreamInitial = true

		case TransportUDPMulticast:
			c.udpReportTimer = time.NewTimer(c.udpReceiverReportPeriod)
			c.checkStreamTimer = time.NewTimer(c.checkStreamPeriod)

		default: // TCP
			c.checkStreamTimer = time.NewTimer(c.checkStreamPeriod)
			c.tcpLastFrameTime = time.Now().Unix()
		}
	} else {
		switch *c.protocol {
		case TransportUDP:
			c.udpReportTimer = time.NewTimer(c.udpSenderReportPeriod)

		case TransportUDPMulticast:
			c.udpReportTimer = time.NewTimer(c.udpSenderReportPeriod)
		}
	}

	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we disable the deadline and perform a check with a ticker.
	c.nconn.SetReadDeadline(time.Time{})

	// start reader
	c.readerErr = make(chan error)
	go func() {
		c.readerErr <- c.runReader()
	}()
}

func (c *Client) runReader() error {
	if *c.protocol == TransportUDP || *c.protocol == TransportUDPMulticast {
		for {
			var res base.Response
			err := res.Read(c.br)
			if err != nil {
				return err
			}
		}
	} else {
		var processFunc func(int, bool, []byte)

		if c.state == clientStatePlay {
			processFunc = func(trackID int, isRTP bool, payload []byte) {
				now := time.Now()
				atomic.StoreInt64(&c.tcpLastFrameTime, now.Unix())

				if isRTP {
					c.tracks[trackID].rtcpReceiver.ProcessPacketRTP(now, payload)
					c.OnPacketRTP(trackID, payload)
				} else {
					c.tracks[trackID].rtcpReceiver.ProcessPacketRTCP(now, payload)
					c.OnPacketRTCP(trackID, payload)
				}
			}
		} else {
			processFunc = func(trackID int, isRTP bool, payload []byte) {
				if !isRTP {
					c.OnPacketRTCP(trackID, payload)
				}
			}
		}

		frame := base.InterleavedFrame{}
		res := base.Response{}

		for {
			frame.Payload = c.tcpReadBuffer.Next()
			what, err := base.ReadInterleavedFrameOrResponse(&frame, &res, c.br)
			if err != nil {
				return err
			}

			if _, ok := what.(*base.InterleavedFrame); ok {
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

				processFunc(trackID, isRTP, frame.Payload)
			}
		}
	}
}

func (c *Client) playRecordStop(isClosing bool) {
	// stop reader
	if c.readerErr != nil {
		c.nconn.SetReadDeadline(time.Now())
		<-c.readerErr
	}

	// stop timers
	c.udpReportTimer = emptyTimer()
	c.checkStreamTimer = emptyTimer()
	c.keepaliveTimer = emptyTimer()

	// forbid writing
	c.writeMutex.Lock()
	c.writeFrameAllowed = false
	c.writeMutex.Unlock()

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
			tlsConfig := c.TLSConfig

			if tlsConfig == nil {
				tlsConfig = &tls.Config{}
			}

			host, _, _ := net.SplitHostPort(c.host)
			tlsConfig.ServerName = host

			return tls.Client(nconn, tlsConfig)
		}
		return nconn
	}()

	c.nconn = nconn
	c.br = bufio.NewReaderSize(conn, clientReadBufferSize)
	c.bw = bufio.NewWriterSize(conn, clientWriteBufferSize)
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
	if c.connCloserDone != nil {
		close(c.connCloserTerminate)
		<-c.connCloserDone
		c.connCloserDone = nil
	}
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
		c.nconn.SetWriteDeadline(time.Now().Add(c.WriteTimeout))
		err := req.Write(c.bw)
		if err != nil {
			return err
		}

		if skipResponse {
			return nil
		}

		c.nconn.SetReadDeadline(time.Now().Add(c.ReadTimeout))

		if c.tcpReadBuffer != nil {
			// read the response and ignore interleaved frames in between;
			// interleaved frames are sent in two scenarios:
			// * when the server is v4lrtspserver, before the PLAY response
			// * when the stream is already playing
			err = res.ReadIgnoreFrames(c.br, c.tcpReadBuffer.Next())
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
			c.reset()

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
		Body: tracks.Write(false),
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
	forPlay bool,
	track *Track,
	baseURL *base.URL,
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

	if (!forPlay && c.state != clientStatePreRecord) ||
		(forPlay && c.state != clientStatePrePlay &&
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
		// protocol set by previous Setup() or trySwitchingProtocol()
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

	mode := headers.TransportModePlay
	if !forPlay {
		mode = headers.TransportModeRecord
	}

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

			return c.doSetup(forPlay, track, baseURL, 0, 0)
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

		if !forPlay || !c.AnyPortEnable {
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
		rtpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtpListener.remotePort = thRes.ServerPorts[0]
		}
		rtpListener.trackID = trackID
		rtpListener.isRTP = true
		cct.udpRTPListener = rtpListener

		rtpListener.remoteWriteAddr = &net.UDPAddr{
			IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
			Zone: rtpListener.remoteZone,
			Port: rtpListener.remotePort,
		}

		rtcpListener.remoteReadIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = c.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtcpListener.remotePort = thRes.ServerPorts[1]
		}
		rtcpListener.trackID = trackID
		rtcpListener.isRTP = false
		cct.udpRTCPListener = rtcpListener

		rtcpListener.remoteWriteAddr = &net.UDPAddr{
			IP:   c.nconn.RemoteAddr().(*net.TCPAddr).IP,
			Zone: rtcpListener.remoteZone,
			Port: rtcpListener.remotePort,
		}

	case TransportUDPMulticast:
		rtpListener.remoteReadIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = ""
		rtpListener.remotePort = thRes.Ports[0]
		rtpListener.trackID = trackID
		rtpListener.isRTP = true
		cct.udpRTPListener = rtpListener

		rtpListener.remoteWriteAddr = &net.UDPAddr{
			IP:   *thRes.Destination,
			Zone: rtpListener.remoteZone,
			Port: rtpListener.remotePort,
		}

		rtcpListener.remoteReadIP = c.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = ""
		rtcpListener.remotePort = thRes.Ports[1]
		rtcpListener.trackID = trackID
		rtcpListener.isRTP = false
		cct.udpRTCPListener = rtcpListener

		rtcpListener.remoteWriteAddr = &net.UDPAddr{
			IP:   *thRes.Destination,
			Zone: rtcpListener.remoteZone,
			Port: rtcpListener.remotePort,
		}

	case TransportTCP:
		if c.tcpReadBuffer == nil {
			c.tcpReadBuffer = multibuffer.New(uint64(c.ReadBufferCount), uint64(c.ReadBufferSize))
		}

		if c.tracksByChannel == nil {
			c.tracksByChannel = make(map[int]int)
		}

		c.tracksByChannel[thRes.InterleavedIDs[0]] = trackID

		cct.tcpChannel = thRes.InterleavedIDs[0]

		cct.tcpRTPFrame = &base.InterleavedFrame{
			Channel: cct.tcpChannel,
		}

		cct.tcpRTCPFrame = &base.InterleavedFrame{
			Channel: cct.tcpChannel + 1,
		}
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
	forPlay bool,
	track *Track,
	baseURL *base.URL,
	rtpPort int,
	rtcpPort int) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case c.setup <- setupReq{
		forPlay:  forPlay,
		track:    track,
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

func (c *Client) doPlay(ra *headers.Range, isSwitchingProtocol bool) (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	c.state = clientStatePlay

	// setup UDP communication before sending the request.
	if *c.protocol == TransportUDP || *c.protocol == TransportUDPMulticast {
		// start UDP listeners
		for _, cct := range c.tracks {
			cct.udpRTPListener.start()
			cct.udpRTCPListener.start()
		}

		// open the firewall by sending packets to the counterpart.
		for _, cct := range c.tracks {
			cct.udpRTPListener.write(
				[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

			cct.udpRTCPListener.write(
				[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
		}
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
		if *c.protocol == TransportUDP || *c.protocol == TransportUDPMulticast {
			// stop UDP listeners
			for _, cct := range c.tracks {
				cct.udpRTPListener.stop()
				cct.udpRTCPListener.stop()
			}
		}

		c.state = clientStatePrePlay

		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	c.lastRange = ra

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

// SetupAndPlay setups and play the given tracks.
func (c *Client) SetupAndPlay(tracks Tracks, baseURL *base.URL) error {
	for _, t := range tracks {
		_, err := c.Setup(true, t, baseURL, 0, 0)
		if err != nil {
			return err
		}
	}

	_, err := c.Play(nil)
	return err
}

func (c *Client) doRecord() (*base.Response, error) {
	err := c.checkState(map[clientState]struct{}{
		clientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	c.state = clientStateRecord

	if *c.protocol == TransportUDP {
		// start UDP listeners
		for _, cct := range c.tracks {
			cct.udpRTPListener.start()
			cct.udpRTCPListener.start()
		}
	}

	res, err := c.do(&base.Request{
		Method: base.Record,
		URL:    c.streamBaseURL,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		if *c.protocol == TransportUDP {
			// stop UDP listeners
			for _, cct := range c.tracks {
				cct.udpRTPListener.stop()
				cct.udpRTCPListener.stop()
			}
		}

		c.state = clientStatePreRecord

		return nil, liberrors.ErrClientBadStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

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

	if *c.protocol == TransportUDP || *c.protocol == TransportUDPMulticast {
		// stop UDP listeners
		for _, cct := range c.tracks {
			cct.udpRTPListener.stop()
			cct.udpRTCPListener.stop()
		}
	}

	// change state regardless of the response
	switch c.state {
	case clientStatePlay:
		c.state = clientStatePrePlay
	case clientStateRecord:
		c.state = clientStatePreRecord
	}

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

// WritePacketRTP writes a RTP packet.
func (c *Client) WritePacketRTP(trackID int, payload []byte) error {
	c.writeMutex.RLock()
	defer c.writeMutex.RUnlock()

	if !c.writeFrameAllowed {
		select {
		case <-c.done:
			return c.closeError
		default:
			return nil
		}
	}

	now := time.Now()

	if c.tracks[trackID].rtcpSender != nil {
		c.tracks[trackID].rtcpSender.ProcessPacketRTP(now, payload)
	}

	switch *c.protocol {
	case TransportUDP, TransportUDPMulticast:
		return c.tracks[trackID].udpRTPListener.write(payload)

	default: // TCP
		f := c.tracks[trackID].tcpRTPFrame

		// a mutex is needed here since bufio.Writer is not thread safe.
		c.tcpWriteMutex.Lock()
		defer c.tcpWriteMutex.Unlock()

		c.nconn.SetWriteDeadline(now.Add(c.WriteTimeout))
		f.Payload = payload
		return f.Write(c.bw)
	}
}

// WritePacketRTCP writes a RTCP packet.
func (c *Client) WritePacketRTCP(trackID int, payload []byte) error {
	c.writeMutex.RLock()
	defer c.writeMutex.RUnlock()

	if !c.writeFrameAllowed {
		select {
		case <-c.done:
			return c.closeError
		default:
			return nil
		}
	}

	now := time.Now()

	switch *c.protocol {
	case TransportUDP, TransportUDPMulticast:
		return c.tracks[trackID].udpRTCPListener.write(payload)

	default: // TCP
		f := c.tracks[trackID].tcpRTCPFrame

		// a mutex is needed here since bufio.Writer is not thread safe.
		c.tcpWriteMutex.Lock()
		defer c.tcpWriteMutex.Unlock()

		c.nconn.SetWriteDeadline(now.Add(c.WriteTimeout))
		f.Payload = payload
		return f.Write(c.bw)
	}
}
