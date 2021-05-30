package gortsplib

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
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
	clientConnReadBufferSize     = 4096
	clientConnWriteBufferSize    = 4096
	clientConnCheckStreamPeriod  = 1 * time.Second
	clientConnUDPKeepalivePeriod = 30 * time.Second
)

func isErrNOUDPPacketsReceivedRecently(err error) bool {
	_, ok := err.(liberrors.ErrClientNoUDPPacketsRecently)
	return ok
}

type clientConnState int

const (
	clientConnStateInitial clientConnState = iota
	clientConnStatePrePlay
	clientConnStatePlay
	clientConnStatePreRecord
	clientConnStateRecord
)

type clientConnTrack struct {
	track           *Track
	udpRTPListener  *clientConnUDPListener
	udpRTCPListener *clientConnUDPListener
	rtcpReceiver    *rtcpreceiver.RTCPReceiver
	rtcpSender      *rtcpsender.RTCPSender
}

func (s clientConnState) String() string {
	switch s {
	case clientConnStateInitial:
		return "initial"
	case clientConnStatePrePlay:
		return "prePlay"
	case clientConnStatePlay:
		return "play"
	case clientConnStatePreRecord:
		return "preRecord"
	case clientConnStateRecord:
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
	tracks Tracks
	res    *base.Response
	err    error
}

// ClientConn is a client-side RTSP connection.
type ClientConn struct {
	c                 *Client
	scheme            string
	host              string
	ctx               context.Context
	ctxCancel         func()
	state             clientConnState
	nconn             net.Conn
	br                *bufio.Reader
	bw                *bufio.Writer
	session           string
	sender            *auth.Sender
	cseq              int
	useGetParameter   bool
	streamBaseURL     *base.URL
	streamProtocol    *StreamProtocol
	tracks            map[int]clientConnTrack
	lastRange         *headers.Range
	backgroundRunning bool
	backgroundErr     error
	tcpFrameBuffer    *multibuffer.MultiBuffer      // tcp
	tcpWriteMutex     sync.Mutex                    // tcp
	readCBMutex       sync.RWMutex                  // read
	readCB            func(int, StreamType, []byte) // read
	writeMutex        sync.RWMutex                  // write
	writeFrameAllowed bool                          // write

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
	readCBSet           chan struct{}
	done                chan struct{}
}

func newClientConn(c *Client, scheme string, host string) (*ClientConn, error) {
	// connection
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 10 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.TLSConfig == nil {
		c.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// reading / writing
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

	cc := &ClientConn{
		c:         c,
		scheme:    scheme,
		host:      host,
		ctx:       ctx,
		ctxCancel: ctxCancel,
		tracks:    make(map[int]clientConnTrack),
		options:   make(chan optionsReq),
		describe:  make(chan describeReq),
		announce:  make(chan announceReq),
		setup:     make(chan setupReq),
		play:      make(chan playReq),
		record:    make(chan recordReq),
		pause:     make(chan pauseReq),
		done:      make(chan struct{}),
	}

	go cc.run()

	return cc, nil
}

// Close closes the connection and waits for all its resources to exit.
func (cc *ClientConn) Close() error {
	cc.ctxCancel()
	<-cc.done
	return nil
}

// Tracks returns all the tracks that the connection is reading or publishing.
func (cc *ClientConn) Tracks() Tracks {
	var ret Tracks

	for _, track := range cc.tracks {
		ret = append(ret, track.track)
	}

	// sort by ID to generate correct SDPs
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].ID < ret[j].ID
	})

	return ret
}

func (cc *ClientConn) run() {
	defer close(cc.done)

outer:
	for {
		select {
		case req := <-cc.options:
			res, err := cc.doOptions(req.url)
			req.res <- clientRes{res: res, err: err}

		case req := <-cc.describe:
			tracks, res, err := cc.doDescribe(req.url)
			req.res <- clientRes{tracks: tracks, res: res, err: err}

		case req := <-cc.announce:
			res, err := cc.doAnnounce(req.url, req.tracks)
			req.res <- clientRes{res: res, err: err}

		case req := <-cc.setup:
			res, err := cc.doSetup(req.mode, req.track, req.rtpPort, req.rtcpPort)
			req.res <- clientRes{res: res, err: err}

		case req := <-cc.play:
			res, err := cc.doPlay(req.ra, false)
			req.res <- clientRes{res: res, err: err}

		case req := <-cc.record:
			res, err := cc.doRecord()
			req.res <- clientRes{res: res, err: err}

		case req := <-cc.pause:
			res, err := cc.doPause()
			req.res <- clientRes{res: res, err: err}

		case err := <-cc.backgroundInnerDone:
			cc.backgroundRunning = false
			err = cc.switchProtocolIfTimeout(err)
			if err != nil {
				cc.backgroundErr = err
				close(cc.backgroundDone)

				cc.writeMutex.Lock()
				cc.writeFrameAllowed = false
				cc.writeMutex.Unlock()
			}

		case <-cc.ctx.Done():
			break outer
		}
	}

	cc.ctxCancel()

	cc.doClose(false)
}

func (cc *ClientConn) doClose(isSwitchingProtocol bool) {
	if cc.backgroundRunning {
		cc.backgroundClose(isSwitchingProtocol)
	}

	if cc.state == clientConnStatePlay || cc.state == clientConnStateRecord {
		cc.do(&base.Request{
			Method: base.Teardown,
			URL:    cc.streamBaseURL,
		}, true)
	}

	for _, track := range cc.tracks {
		if track.udpRTPListener != nil {
			track.udpRTPListener.close()
			track.udpRTCPListener.close()
		}
	}

	if cc.nconn != nil {
		cc.nconn.Close()
		cc.nconn = nil
	}
}

func (cc *ClientConn) reset(isSwitchingProtocol bool) {
	cc.doClose(isSwitchingProtocol)

	cc.state = clientConnStateInitial
	cc.session = ""
	cc.sender = nil
	cc.cseq = 0
	cc.useGetParameter = false
	cc.streamBaseURL = nil
	cc.streamProtocol = nil
	cc.tracks = make(map[int]clientConnTrack)
	cc.tcpFrameBuffer = nil

	if !isSwitchingProtocol {
		cc.readCB = nil
	}
}

func (cc *ClientConn) checkState(allowed map[clientConnState]struct{}) error {
	if _, ok := allowed[cc.state]; ok {
		return nil
	}

	allowedList := make([]fmt.Stringer, len(allowed))
	i := 0
	for a := range allowed {
		allowedList[i] = a
		i++
	}
	return liberrors.ErrClientWrongState{AllowedList: allowedList, State: cc.state}
}

func (cc *ClientConn) switchProtocolIfTimeout(err error) error {
	if *cc.streamProtocol != StreamProtocolUDP ||
		cc.state != clientConnStatePlay ||
		!isErrNOUDPPacketsReceivedRecently(err) ||
		cc.c.StreamProtocol != nil {
		return err
	}

	prevBaseURL := cc.streamBaseURL
	oldUseGetParameter := cc.useGetParameter
	prevTracks := cc.tracks

	cc.reset(true)

	v := StreamProtocolTCP
	cc.streamProtocol = &v
	cc.useGetParameter = oldUseGetParameter
	cc.scheme = prevBaseURL.Scheme
	cc.host = prevBaseURL.Host

	err = cc.connOpen()
	if err != nil {
		return err
	}

	for _, track := range prevTracks {
		_, err := cc.doSetup(headers.TransportModePlay, track.track, 0, 0)
		if err != nil {
			return err
		}
	}

	_, err = cc.doPlay(cc.lastRange, true)
	if err != nil {
		return err
	}

	return nil
}

func (cc *ClientConn) pullReadCB() func(int, StreamType, []byte) {
	cc.readCBMutex.RLock()
	defer cc.readCBMutex.RUnlock()
	return cc.readCB
}

func (cc *ClientConn) backgroundStart(isSwitchingProtocol bool) {
	cc.writeMutex.Lock()
	cc.writeFrameAllowed = true
	cc.writeMutex.Unlock()

	cc.backgroundRunning = true
	cc.backgroundTerminate = make(chan struct{})
	cc.backgroundInnerDone = make(chan error)

	if !isSwitchingProtocol {
		cc.backgroundDone = make(chan struct{})
	}

	go cc.runBackground()
}

func (cc *ClientConn) backgroundClose(isSwitchingProtocol bool) {
	close(cc.backgroundTerminate)
	err := <-cc.backgroundInnerDone
	cc.backgroundRunning = false

	if !isSwitchingProtocol {
		cc.backgroundErr = err
		close(cc.backgroundDone)
	}

	cc.writeMutex.Lock()
	cc.writeFrameAllowed = false
	cc.writeMutex.Unlock()
}

func (cc *ClientConn) runBackground() {
	cc.backgroundInnerDone <- func() error {
		if cc.state == clientConnStatePlay {
			if *cc.streamProtocol == StreamProtocolUDP {
				return cc.runBackgroundPlayUDP()
			}
			return cc.runBackgroundPlayTCP()
		}

		if *cc.streamProtocol == StreamProtocolUDP {
			return cc.runBackgroundRecordUDP()
		}
		return cc.runBackgroundRecordTCP()
	}()
}

func (cc *ClientConn) runBackgroundPlayUDP() error {
	// open the firewall by sending packets to the counterpart
	for _, cct := range cc.tracks {
		cct.udpRTPListener.write(
			[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

		cct.udpRTCPListener.write(
			[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
	}

	for _, cct := range cc.tracks {
		cct.udpRTPListener.start()
		cct.udpRTCPListener.start()
	}

	defer func() {
		for _, cct := range cc.tracks {
			cct.udpRTPListener.stop()
			cct.udpRTCPListener.stop()
		}
	}()

	// disable deadline
	cc.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			var res base.Response
			err := res.Read(cc.br)
			if err != nil {
				readerDone <- err
				return
			}
		}
	}()

	reportTicker := time.NewTicker(cc.c.receiverReportPeriod)
	defer reportTicker.Stop()

	keepaliveTicker := time.NewTicker(clientConnUDPKeepalivePeriod)
	defer keepaliveTicker.Stop()

	checkStreamInitial := true
	checkStreamTicker := time.NewTicker(cc.c.InitialUDPReadTimeout)
	defer func() {
		checkStreamTicker.Stop()
	}()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				rr := cct.rtcpReceiver.Report(now)
				cc.WriteFrame(trackID, StreamTypeRTCP, rr)
			}

		case <-keepaliveTicker.C:
			_, err := cc.do(&base.Request{
				Method: func() base.Method {
					// the vlc integrated rtsp server requires GET_PARAMETER
					if cc.useGetParameter {
						return base.GetParameter
					}
					return base.Options
				}(),
				// use the stream base URL, otherwise some cameras do not reply
				URL: cc.streamBaseURL,
			}, true)
			if err != nil {
				cc.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return err
			}

		case <-checkStreamTicker.C:
			if checkStreamInitial {
				// check that at least one packet has been received
				inTimeout := func() bool {
					for _, cct := range cc.tracks {
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
					cc.nconn.SetReadDeadline(time.Now())
					<-readerDone
					return liberrors.ErrClientNoUDPPacketsRecently{}
				}

				checkStreamInitial = false
				checkStreamTicker.Stop()
				checkStreamTicker = time.NewTicker(clientConnCheckStreamPeriod)

			} else {
				inTimeout := func() bool {
					now := time.Now()
					for _, cct := range cc.tracks {
						lft := time.Unix(atomic.LoadInt64(cct.udpRTPListener.lastFrameTime), 0)
						if now.Sub(lft) < cc.c.ReadTimeout {
							return false
						}

						lft = time.Unix(atomic.LoadInt64(cct.udpRTCPListener.lastFrameTime), 0)
						if now.Sub(lft) < cc.c.ReadTimeout {
							return false
						}
					}
					return true
				}()
				if inTimeout {
					cc.nconn.SetReadDeadline(time.Now())
					<-readerDone
					return liberrors.ErrClientUDPTimeout{}
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (cc *ClientConn) runBackgroundPlayTCP() error {
	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we disable the deadline and perform check with a ticker.
	cc.nconn.SetReadDeadline(time.Time{})

	lastFrameTime := time.Now().Unix()

	readerDone := make(chan error)
	go func() {
		for {
			frame := base.InterleavedFrame{
				Payload: cc.tcpFrameBuffer.Next(),
			}
			err := frame.Read(cc.br)
			if err != nil {
				readerDone <- err
				return
			}

			track, ok := cc.tracks[frame.TrackID]
			if !ok {
				continue
			}

			now := time.Now()
			atomic.StoreInt64(&lastFrameTime, now.Unix())
			track.rtcpReceiver.ProcessFrame(now, frame.StreamType, frame.Payload)
			cc.pullReadCB()(frame.TrackID, frame.StreamType, frame.Payload)
		}
	}()

	reportTicker := time.NewTicker(cc.c.receiverReportPeriod)
	defer reportTicker.Stop()

	checkStreamTicker := time.NewTicker(clientConnCheckStreamPeriod)
	defer checkStreamTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				rr := cct.rtcpReceiver.Report(now)
				cc.WriteFrame(trackID, StreamTypeRTCP, rr)
			}

		case <-checkStreamTicker.C:
			inTimeout := func() bool {
				now := time.Now()
				lft := time.Unix(atomic.LoadInt64(&lastFrameTime), 0)
				return now.Sub(lft) >= cc.c.ReadTimeout
			}()
			if inTimeout {
				cc.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return liberrors.ErrClientTCPTimeout{}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (cc *ClientConn) runBackgroundRecordUDP() error {
	for _, cct := range cc.tracks {
		cct.udpRTPListener.start()
		cct.udpRTCPListener.start()
	}

	defer func() {
		for _, cct := range cc.tracks {
			cct.udpRTPListener.stop()
			cct.udpRTCPListener.stop()
		}
	}()

	// disable deadline
	cc.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			var res base.Response
			err := res.Read(cc.br)
			if err != nil {
				readerDone <- err
				return
			}
		}
	}()

	reportTicker := time.NewTicker(cc.c.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					cc.WriteFrame(trackID, StreamTypeRTCP, sr)
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (cc *ClientConn) runBackgroundRecordTCP() error {
	// disable deadline
	cc.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			frame := base.InterleavedFrame{
				Payload: cc.tcpFrameBuffer.Next(),
			}
			err := frame.Read(cc.br)
			if err != nil {
				readerDone <- err
				return
			}

			cc.pullReadCB()(frame.TrackID, frame.StreamType, frame.Payload)
		}
	}()

	reportTicker := time.NewTicker(cc.c.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					cc.WriteFrame(trackID, StreamTypeRTCP, sr)
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (cc *ClientConn) connOpen() error {
	if cc.scheme != "rtsp" && cc.scheme != "rtsps" {
		return fmt.Errorf("unsupported scheme '%s'", cc.scheme)
	}

	v := StreamProtocolUDP
	if cc.scheme == "rtsps" && cc.c.StreamProtocol == &v {
		return fmt.Errorf("RTSPS can't be used with UDP")
	}

	if !strings.Contains(cc.host, ":") {
		cc.host += ":554"
	}

	ctx, cancel := context.WithTimeout(cc.ctx, cc.c.ReadTimeout)
	defer cancel()

	nconn, err := cc.c.DialContext(ctx, "tcp", cc.host)
	if err != nil {
		return err
	}

	conn := func() net.Conn {
		if cc.scheme == "rtsps" {
			return tls.Client(nconn, cc.c.TLSConfig)
		}
		return nconn
	}()

	cc.nconn = nconn
	cc.br = bufio.NewReaderSize(conn, clientConnReadBufferSize)
	cc.bw = bufio.NewWriterSize(conn, clientConnWriteBufferSize)
	return nil
}

func (cc *ClientConn) do(req *base.Request, skipResponse bool) (*base.Response, error) {
	if cc.nconn == nil {
		err := cc.connOpen()
		if err != nil {
			return nil, err
		}
	}

	if req.Header == nil {
		req.Header = make(base.Header)
	}

	if cc.session != "" {
		req.Header["Session"] = base.HeaderValue{cc.session}
	}

	if cc.sender != nil {
		cc.sender.AddAuthorization(req)
	}

	cc.cseq++
	req.Header["CSeq"] = base.HeaderValue{strconv.FormatInt(int64(cc.cseq), 10)}

	req.Header["User-Agent"] = base.HeaderValue{"gortsplib"}

	if cc.c.OnRequest != nil {
		cc.c.OnRequest(req)
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
				case <-cc.ctx.Done():
					cc.nconn.Close()
				case <-ctxHandlerTerminate:
				}
			}()
		}

		cc.nconn.SetWriteDeadline(time.Now().Add(cc.c.WriteTimeout))
		err := req.Write(cc.bw)
		if err != nil {
			return err
		}

		if skipResponse {
			return nil
		}

		cc.nconn.SetReadDeadline(time.Now().Add(cc.c.ReadTimeout))

		if cc.tcpFrameBuffer != nil {
			// read the response and ignore interleaved frames in between;
			// interleaved frames are sent in two scenarios:
			// * when the server is v4lrtspserver, before the PLAY response
			// * when the stream is already playing
			err = res.ReadIgnoreFrames(cc.br, cc.tcpFrameBuffer.Next())
			if err != nil {
				return err
			}
		} else {
			err = res.Read(cc.br)
			if err != nil {
				return err
			}
		}

		return nil
	}()
	if err != nil {
		return nil, err
	}

	if cc.c.OnResponse != nil {
		cc.c.OnResponse(&res)
	}

	// get session from response
	if v, ok := res.Header["Session"]; ok {
		var sx headers.Session
		err := sx.Read(v)
		if err != nil {
			return nil, liberrors.ErrClientSessionHeaderInvalid{Err: err}
		}
		cc.session = sx.Session
	}

	// if required, send request again with authentication
	if res.StatusCode == base.StatusUnauthorized && req.URL.User != nil && cc.sender == nil {
		pass, _ := req.URL.User.Password()
		user := req.URL.User.Username()

		sender, err := auth.NewSender(res.Header["WWW-Authenticate"], user, pass)
		if err != nil {
			return nil, fmt.Errorf("unable to setup authentication: %s", err)
		}
		cc.sender = sender

		return cc.do(req, false)
	}

	return &res, nil
}

func (cc *ClientConn) doOptions(u *base.URL) (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := cc.do(&base.Request{
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
		return res, liberrors.ErrClientWrongStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	cc.useGetParameter = func() bool {
		pub, ok := res.Header["Public"]
		if !ok || len(pub) != 1 {
			return false
		}

		for _, m := range strings.Split(pub[0], ",") {
			if base.Method(m) == base.GetParameter {
				return true
			}
		}
		return false
	}()

	return res, nil
}

// Options writes an OPTIONS request and reads a response.
func (cc *ClientConn) Options(u *base.URL) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case cc.options <- optionsReq{url: u, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-cc.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (cc *ClientConn) doDescribe(u *base.URL) (Tracks, *base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, nil, err
	}

	res, err := cc.do(&base.Request{
		Method: base.Describe,
		URL:    u,
		Header: base.Header{
			"Accept": base.HeaderValue{"application/sdp"},
		},
	}, false)
	if err != nil {
		return nil, nil, err
	}

	if res.StatusCode != base.StatusOK {
		// redirect
		if !cc.c.RedirectDisable &&
			res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {

			cc.reset(false)

			u, err := base.ParseURL(res.Header["Location"][0])
			if err != nil {
				return nil, nil, err
			}

			cc.scheme = u.Scheme
			cc.host = u.Host

			err = cc.connOpen()
			if err != nil {
				return nil, nil, err
			}

			_, err = cc.doOptions(u)
			if err != nil {
				return nil, nil, err
			}

			return cc.doDescribe(u)
		}

		return nil, res, liberrors.ErrClientWrongStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	ct, ok := res.Header["Content-Type"]
	if !ok || len(ct) != 1 {
		return nil, nil, liberrors.ErrClientContentTypeMissing{}
	}

	if ct[0] != "application/sdp" {
		return nil, nil, liberrors.ErrClientContentTypeUnsupported{CT: ct}
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
		return nil, nil, err
	}

	tracks, err := ReadTracks(res.Body, baseURL)
	if err != nil {
		return nil, nil, err
	}

	return tracks, res, nil
}

// Describe writes a DESCRIBE request and reads a Response.
func (cc *ClientConn) Describe(u *base.URL) (Tracks, *base.Response, error) {
	cres := make(chan clientRes)
	select {
	case cc.describe <- describeReq{url: u, res: cres}:
		res := <-cres
		return res.tracks, res.res, res.err

	case <-cc.ctx.Done():
		return nil, nil, liberrors.ErrClientTerminated{}
	}
}

func (cc *ClientConn) doAnnounce(u *base.URL, tracks Tracks) (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStateInitial: {},
	})
	if err != nil {
		return nil, err
	}

	// in case of ANNOUNCE, the base URL doesn't have a trailing slash.
	// (tested with ffmpeg and gstreamer)
	baseURL := u.Clone()

	// set id, base url and control attribute on tracks
	for i, t := range tracks {
		t.ID = i
		t.BaseURL = baseURL
		t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})
	}

	res, err := cc.do(&base.Request{
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
		return nil, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	cc.streamBaseURL = baseURL
	cc.state = clientConnStatePreRecord

	return res, nil
}

// Announce writes an ANNOUNCE request and reads a Response.
func (cc *ClientConn) Announce(u *base.URL, tracks Tracks) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case cc.announce <- announceReq{url: u, tracks: tracks, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-cc.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (cc *ClientConn) doSetup(
	mode headers.TransportMode,
	track *Track,
	rtpPort int,
	rtcpPort int) (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStateInitial:   {},
		clientConnStatePrePlay:   {},
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	if (mode == headers.TransportModeRecord && cc.state != clientConnStatePreRecord) ||
		(mode == headers.TransportModePlay && cc.state != clientConnStatePrePlay &&
			cc.state != clientConnStateInitial) {
		return nil, liberrors.ErrClientCannotReadPublishAtSameTime{}
	}

	if cc.streamBaseURL != nil && *track.BaseURL != *cc.streamBaseURL {
		return nil, liberrors.ErrClientCannotSetupTracksDifferentURLs{}
	}

	var rtpListener *clientConnUDPListener
	var rtcpListener *clientConnUDPListener

	// always use TCP if encrypted
	if cc.scheme == "rtsps" {
		v := StreamProtocolTCP
		cc.streamProtocol = &v
	}

	proto := func() StreamProtocol {
		// protocol set by previous Setup() or switchProtocolIfTimeout()
		if cc.streamProtocol != nil {
			return *cc.streamProtocol
		}

		// protocol set by conf
		if cc.c.StreamProtocol != nil {
			return *cc.c.StreamProtocol
		}

		// try UDP
		return StreamProtocolUDP
	}()

	th := headers.Transport{
		Protocol: proto,
		Delivery: func() *base.StreamDelivery {
			v := base.StreamDeliveryUnicast
			return &v
		}(),
		Mode: &mode,
	}

	if proto == base.StreamProtocolUDP {
		if (rtpPort == 0 && rtcpPort != 0) ||
			(rtpPort != 0 && rtcpPort == 0) {
			return nil, liberrors.ErrClientUDPPortsZero{}
		}

		if rtpPort != 0 && rtcpPort != (rtpPort+1) {
			return nil, liberrors.ErrClientUDPPortsNotConsecutive{}
		}

		var err error
		rtpListener, rtcpListener, err = func() (*clientConnUDPListener, *clientConnUDPListener, error) {
			if rtpPort != 0 {
				rtpListener, err := newClientConnUDPListener(cc, rtpPort)
				if err != nil {
					return nil, nil, err
				}

				rtcpListener, err := newClientConnUDPListener(cc, rtcpPort)
				if err != nil {
					rtpListener.close()
					return nil, nil, err
				}

				return rtpListener, rtcpListener, nil
			}

			// choose two consecutive ports in range 65535-10000
			// rtp must be even and rtcp odd
			for {
				rtpPort = (rand.Intn((65535-10000)/2) * 2) + 10000
				rtcpPort = rtpPort + 1

				rtpListener, err := newClientConnUDPListener(cc, rtpPort)
				if err != nil {
					continue
				}

				rtcpListener, err := newClientConnUDPListener(cc, rtcpPort)
				if err != nil {
					rtpListener.close()
					continue
				}

				return rtpListener, rtcpListener, nil
			}
		}()
		if err != nil {
			return nil, err
		}

		th.ClientPorts = &[2]int{rtpPort, rtcpPort}

	} else {
		th.InterleavedIDs = &[2]int{(track.ID * 2), (track.ID * 2) + 1}
	}

	trackURL, err := track.URL()
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, err
	}

	res, err := cc.do(&base.Request{
		Method: base.Setup,
		URL:    trackURL,
		Header: base.Header{
			"Transport": th.Write(),
		},
	}, false)
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}

		// switch protocol automatically
		if res.StatusCode == base.StatusUnsupportedTransport &&
			cc.streamProtocol == nil &&
			cc.c.StreamProtocol == nil {

			v := StreamProtocolTCP
			cc.streamProtocol = &v

			return cc.doSetup(mode, track, 0, 0)
		}

		return res, liberrors.ErrClientWrongStatusCode{Code: res.StatusCode, Message: res.StatusMessage}
	}

	var thRes headers.Transport
	err = thRes.Read(res.Header["Transport"])
	if err != nil {
		if proto == StreamProtocolUDP {
			rtpListener.close()
			rtcpListener.close()
		}
		return nil, liberrors.ErrClientTransportHeaderInvalid{Err: err}
	}

	if proto == StreamProtocolUDP {
		if thRes.ServerPorts != nil {
			if (thRes.ServerPorts[0] == 0 && thRes.ServerPorts[1] != 0) ||
				(thRes.ServerPorts[0] != 0 && thRes.ServerPorts[1] == 0) {
				rtpListener.close()
				rtcpListener.close()
				return nil, liberrors.ErrClientServerPortsZero{}
			}
		}

		if !cc.c.AnyPortEnable {
			if thRes.ServerPorts == nil || (thRes.ServerPorts[0] == 0 && thRes.ServerPorts[1] == 0) {
				rtpListener.close()
				rtcpListener.close()
				return nil, liberrors.ErrClientServerPortsNotProvided{}
			}
		}

	} else {
		if thRes.InterleavedIDs == nil {
			return nil, liberrors.ErrClientTransportHeaderNoInterleavedIDs{}
		}

		if thRes.InterleavedIDs[0] != th.InterleavedIDs[0] ||
			thRes.InterleavedIDs[1] != th.InterleavedIDs[1] {
			return nil, liberrors.ErrClientTransportHeaderWrongInterleavedIDs{
				Expected: *th.InterleavedIDs, Value: *thRes.InterleavedIDs,
			}
		}
	}

	clockRate, _ := track.ClockRate()
	cct := clientConnTrack{
		track: track,
	}

	if mode == headers.TransportModePlay {
		cct.rtcpReceiver = rtcpreceiver.New(nil, clockRate)
	} else {
		cct.rtcpSender = rtcpsender.New(clockRate)
	}

	cc.streamBaseURL = track.BaseURL
	cc.streamProtocol = &proto

	if proto == StreamProtocolUDP {
		rtpListener.remoteIP = cc.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtpListener.remoteZone = cc.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtpListener.remotePort = thRes.ServerPorts[0]
		}
		rtpListener.trackID = track.ID
		rtpListener.streamType = StreamTypeRTP
		cct.udpRTPListener = rtpListener

		rtcpListener.remoteIP = cc.nconn.RemoteAddr().(*net.TCPAddr).IP
		rtcpListener.remoteZone = cc.nconn.RemoteAddr().(*net.TCPAddr).Zone
		if thRes.ServerPorts != nil {
			rtcpListener.remotePort = thRes.ServerPorts[1]
		}
		rtcpListener.trackID = track.ID
		rtcpListener.streamType = StreamTypeRTCP
		cct.udpRTCPListener = rtcpListener
	}

	cc.tracks[track.ID] = cct

	if mode == headers.TransportModePlay {
		cc.state = clientConnStatePrePlay
	} else {
		cc.state = clientConnStatePreRecord
	}

	if *cc.streamProtocol == StreamProtocolTCP &&
		cc.tcpFrameBuffer == nil {
		cc.tcpFrameBuffer = multibuffer.New(uint64(cc.c.ReadBufferCount), uint64(cc.c.ReadBufferSize))
	}

	return res, nil
}

// Setup writes a SETUP request and reads a Response.
// rtpPort and rtcpPort are used only if protocol is UDP.
// if rtpPort and rtcpPort are zero, they are chosen automatically.
func (cc *ClientConn) Setup(
	mode headers.TransportMode,
	track *Track,
	rtpPort int,
	rtcpPort int) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case cc.setup <- setupReq{
		mode:     mode,
		track:    track,
		rtpPort:  rtpPort,
		rtcpPort: rtcpPort,
		res:      cres,
	}:
		res := <-cres
		return res.res, res.err

	case <-cc.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (cc *ClientConn) doPlay(ra *headers.Range, isSwitchingProtocol bool) (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	header := make(base.Header)

	if ra != nil {
		header["Range"] = ra.Write()
	}

	res, err := cc.do(&base.Request{
		Method: base.Play,
		URL:    cc.streamBaseURL,
		Header: header,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	cc.state = clientConnStatePlay
	cc.lastRange = ra

	if !isSwitchingProtocol {
		// use a temporary callback that is replaces as soon as
		// the user calls ReadFrames()
		cc.readCBSet = make(chan struct{})
		copy := cc.readCBSet
		cc.readCB = func(trackID int, streamType base.StreamType, payload []byte) {
			select {
			case <-copy:
			case <-cc.ctx.Done():
				return
			}
			cc.pullReadCB()(trackID, streamType, payload)
		}
	}

	cc.backgroundStart(isSwitchingProtocol)

	return res, nil
}

// Play writes a PLAY request and reads a Response.
// This can be called only after Setup().
func (cc *ClientConn) Play(ra *headers.Range) (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case cc.play <- playReq{ra: ra, res: cres}:
		res := <-cres
		return res.res, res.err

	case <-cc.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (cc *ClientConn) doRecord() (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := cc.do(&base.Request{
		Method: base.Record,
		URL:    cc.streamBaseURL,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	cc.state = clientConnStateRecord

	// when publishing, calling ReadFrames() is not mandatory
	// use an empty callback
	cc.readCB = func(trackID int, streamType base.StreamType, payload []byte) {
	}

	cc.backgroundStart(false)

	return nil, nil
}

// Record writes a RECORD request and reads a Response.
// This can be called only after Announce() and Setup().
func (cc *ClientConn) Record() (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case cc.record <- recordReq{res: cres}:
		res := <-cres
		return res.res, res.err

	case <-cc.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

func (cc *ClientConn) doPause() (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePlay:   {},
		clientConnStateRecord: {},
	})
	if err != nil {
		return nil, err
	}

	cc.backgroundClose(false)

	res, err := cc.do(&base.Request{
		Method: base.Pause,
		URL:    cc.streamBaseURL,
	}, false)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return res, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage,
		}
	}

	switch cc.state {
	case clientConnStatePlay:
		cc.state = clientConnStatePrePlay
	case clientConnStateRecord:
		cc.state = clientConnStatePreRecord
	}

	return res, nil
}

// Pause writes a PAUSE request and reads a Response.
// This can be called only after Play() or Record().
func (cc *ClientConn) Pause() (*base.Response, error) {
	cres := make(chan clientRes)
	select {
	case cc.pause <- pauseReq{res: cres}:
		res := <-cres
		return res.res, res.err

	case <-cc.ctx.Done():
		return nil, liberrors.ErrClientTerminated{}
	}
}

// Seek asks the server to re-start the stream from a specific timestamp.
func (cc *ClientConn) Seek(ra *headers.Range) (*base.Response, error) {
	_, err := cc.Pause()
	if err != nil {
		return nil, err
	}

	return cc.Play(ra)
}

// ReadFrames starts reading frames.
func (cc *ClientConn) ReadFrames(onFrame func(int, StreamType, []byte)) error {
	cc.readCBMutex.Lock()
	cc.readCB = onFrame
	cc.readCBMutex.Unlock()

	// replace temporary callback with final callback
	if cc.readCBSet != nil {
		close(cc.readCBSet)
		cc.readCBSet = nil
	}

	<-cc.backgroundDone
	return cc.backgroundErr
}

// WriteFrame writes a frame.
func (cc *ClientConn) WriteFrame(trackID int, streamType StreamType, payload []byte) error {
	now := time.Now()

	cc.writeMutex.RLock()
	defer cc.writeMutex.RUnlock()

	if !cc.writeFrameAllowed {
		return cc.backgroundErr
	}

	if cc.tracks[trackID].rtcpSender != nil {
		cc.tracks[trackID].rtcpSender.ProcessFrame(now, streamType, payload)
	}

	if *cc.streamProtocol == StreamProtocolUDP {
		if streamType == StreamTypeRTP {
			return cc.tracks[trackID].udpRTPListener.write(payload)
		}
		return cc.tracks[trackID].udpRTCPListener.write(payload)
	}

	cc.tcpWriteMutex.Lock()
	defer cc.tcpWriteMutex.Unlock()

	cc.nconn.SetWriteDeadline(now.Add(cc.c.WriteTimeout))
	return base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Payload:    payload,
	}.Write(cc.bw)
}
