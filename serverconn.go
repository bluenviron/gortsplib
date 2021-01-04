package gortsplib

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/multibuffer"
)

const (
	serverReadBufferSize  = 4096
	serverWriteBufferSize = 4096
)

// server errors.
var (
	ErrServerTeardown = errors.New("teardown")
)

type serverConnState int

const (
	serverConnStateInitial serverConnState = iota
	serverConnStatePlay
	serverConnStateRecord
)

type serverConnTrack struct {
	proto    StreamProtocol
	rtpPort  int
	rtcpPort int
}

func extractTrackID(controlPath string, mode *headers.TransportMode, trackLen int) (int, error) {
	if mode == nil || *mode == headers.TransportModePlay {
		if !strings.HasPrefix(controlPath, "trackID=") {
			return 0, fmt.Errorf("invalid control attribute (%s)", controlPath)
		}

		tmp, err := strconv.ParseInt(controlPath[len("trackID="):], 10, 64)
		if err != nil || tmp < 0 {
			return 0, fmt.Errorf("invalid track id (%s)", controlPath)
		}
		trackID := int(tmp)

		return trackID, nil
	}

	return trackLen, nil
}

// ServerConnReadHandlers allows to set the handlers required by ServerConn.Read.
// all fields are optional.
type ServerConnReadHandlers struct {
	// called after receiving any request.
	OnRequest func(req *base.Request)

	// called before sending any response.
	OnResponse func(res *base.Response)

	// called after receiving a OPTIONS request.
	// if nil, it is generated automatically.
	OnOptions func(req *base.Request) (*base.Response, error)

	// called after receiving a DESCRIBE request.
	OnDescribe func(req *base.Request) (*base.Response, error)

	// called after receiving an ANNOUNCE request.
	OnAnnounce func(req *base.Request, tracks Tracks) (*base.Response, error)

	// called after receiving a SETUP request.
	OnSetup func(req *base.Request, th *headers.Transport) (*base.Response, error)

	// called after receiving a PLAY request.
	OnPlay func(req *base.Request) (*base.Response, error)

	// called after receiving a RECORD request.
	OnRecord func(req *base.Request) (*base.Response, error)

	// called after receiving a PAUSE request.
	OnPause func(req *base.Request) (*base.Response, error)

	// called after receiving a GET_PARAMETER request.
	// if nil, it is generated automatically.
	OnGetParameter func(req *base.Request) (*base.Response, error)

	// called after receiving a SET_PARAMETER request.
	OnSetParameter func(req *base.Request) (*base.Response, error)

	// called after receiving a TEARDOWN request.
	// if nil, it is generated automatically.
	OnTeardown func(req *base.Request) (*base.Response, error)

	// called after receiving a Frame.
	OnFrame func(trackID int, streamType StreamType, payload []byte)
}

// ServerConn is a server-side RTSP connection.
type ServerConn struct {
	s                  *Server
	nconn              net.Conn
	br                 *bufio.Reader
	bw                 *bufio.Writer
	state              serverConnState
	tracks             map[int]serverConnTrack
	tracksProto        *StreamProtocol
	writeMutex         sync.Mutex
	readHandlers       ServerConnReadHandlers
	nextFramesEnabled  bool
	framesEnabled      bool
	readTimeoutEnabled bool

	// in
	terminate chan struct{}
}

func newServerConn(s *Server, nconn net.Conn) *ServerConn {
	conn := func() net.Conn {
		if s.conf.TLSConfig != nil {
			return tls.Server(nconn, s.conf.TLSConfig)
		}
		return nconn
	}()

	return &ServerConn{
		s:         s,
		nconn:     nconn,
		br:        bufio.NewReaderSize(conn, serverReadBufferSize),
		bw:        bufio.NewWriterSize(conn, serverWriteBufferSize),
		tracks:    make(map[int]serverConnTrack),
		terminate: make(chan struct{}),
	}
}

// Close closes all the connection resources.
func (sc *ServerConn) Close() error {
	err := sc.nconn.Close()
	close(sc.terminate)
	return err
}

// NetConn returns the underlying net.Conn.
func (sc *ServerConn) NetConn() net.Conn {
	return sc.nconn
}

func (sc *ServerConn) ip() net.IP {
	return sc.nconn.RemoteAddr().(*net.TCPAddr).IP
}

func (sc *ServerConn) zone() string {
	return sc.nconn.RemoteAddr().(*net.TCPAddr).Zone
}

func (sc *ServerConn) frameModeEnable() {
	switch sc.state {
	case serverConnStatePlay:
		if *sc.tracksProto == StreamProtocolTCP {
			sc.nextFramesEnabled = true
		}

	case serverConnStateRecord:
		if *sc.tracksProto == StreamProtocolTCP {
			sc.nextFramesEnabled = true
			sc.readTimeoutEnabled = true

		} else {
			for trackID, track := range sc.tracks {
				sc.s.conf.UDPRTPListener.addPublisher(sc.ip(), track.rtpPort, trackID, sc)
				sc.s.conf.UDPRTCPListener.addPublisher(sc.ip(), track.rtcpPort, trackID, sc)
			}
		}
	}
}

func (sc *ServerConn) frameModeDisable() {
	switch sc.state {
	case serverConnStatePlay:
		sc.nextFramesEnabled = false

	case serverConnStateRecord:
		sc.nextFramesEnabled = false
		sc.readTimeoutEnabled = false

		for _, track := range sc.tracks {
			if track.proto == StreamProtocolUDP {
				sc.s.conf.UDPRTPListener.removePublisher(sc.ip(), track.rtpPort)
				sc.s.conf.UDPRTCPListener.removePublisher(sc.ip(), track.rtcpPort)
			}
		}
	}
}

func (sc *ServerConn) handleRequest(req *base.Request) (*base.Response, error) {
	if sc.readHandlers.OnRequest != nil {
		sc.readHandlers.OnRequest(req)
	}

	switch req.Method {
	case base.Options:
		if sc.readHandlers.OnOptions != nil {
			return sc.readHandlers.OnOptions(req)
		}

		var methods []string
		if sc.readHandlers.OnDescribe != nil {
			methods = append(methods, string(base.Describe))
		}
		if sc.readHandlers.OnAnnounce != nil {
			methods = append(methods, string(base.Announce))
		}
		if sc.readHandlers.OnSetup != nil {
			methods = append(methods, string(base.Setup))
		}
		if sc.readHandlers.OnPlay != nil {
			methods = append(methods, string(base.Play))
		}
		if sc.readHandlers.OnRecord != nil {
			methods = append(methods, string(base.Record))
		}
		if sc.readHandlers.OnPause != nil {
			methods = append(methods, string(base.Pause))
		}
		methods = append(methods, string(base.GetParameter))
		if sc.readHandlers.OnSetParameter != nil {
			methods = append(methods, string(base.SetParameter))
		}
		methods = append(methods, string(base.Teardown))

		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join(methods, ", ")},
			},
		}, nil

	case base.Describe:
		if sc.readHandlers.OnDescribe != nil {
			return sc.readHandlers.OnDescribe(req)
		}

	case base.Announce:
		if sc.readHandlers.OnAnnounce != nil {
			ct, ok := req.Header["Content-Type"]
			if !ok || len(ct) != 1 {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, errors.New("Content-Type header is missing")
			}

			if ct[0] != "application/sdp" {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("unsupported Content-Type '%s'", ct)
			}

			tracks, err := ReadTracks(req.Content)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("invalid SDP: %s", err)
			}

			if len(tracks) == 0 {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, errors.New("no tracks defined")
			}

			res, err := sc.readHandlers.OnAnnounce(req, tracks)
			return res, err
		}

	case base.Setup:
		if sc.readHandlers.OnSetup != nil {
			_, controlPath, ok := req.URL.BasePathControlAttr()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("unable to find control attribute (%s)", req.URL)
			}

			th, err := headers.ReadTransport(req.Header["Transport"])
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("transport header: %s", err)
			}

			trackID, err := extractTrackID(controlPath, th.Mode, len(sc.tracks))
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, err
			}

			if _, ok := sc.tracks[trackID]; ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("track %d has already been setup", trackID)
			}

			if sc.tracksProto != nil && *sc.tracksProto != th.Protocol {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("can't receive tracks with different protocols")
			}

			if th.Protocol == StreamProtocolUDP {
				if sc.s.conf.UDPRTPListener == nil {
					return &base.Response{
						StatusCode: base.StatusUnsupportedTransport,
					}, nil
				}

				if th.ClientPorts == nil {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, fmt.Errorf("transport header does not have valid client ports (%v)", req.Header["Transport"])
				}

			} else {
				if th.InterleavedIds == nil {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, fmt.Errorf("transport header does not contain the interleaved field")
				}

				if (*th.InterleavedIds)[0] != (trackID*2) ||
					(*th.InterleavedIds)[1] != (1+trackID*2) {
					return &base.Response{
							StatusCode: base.StatusBadRequest,
						}, fmt.Errorf("wrong interleaved ids, expected [%v %v], got %v",
							(trackID * 2), (1 + trackID*2), *th.InterleavedIds)
				}
			}

			res, err := sc.readHandlers.OnSetup(req, th)

			if res.StatusCode == 200 {
				sc.tracksProto = &th.Protocol

				if th.Protocol == StreamProtocolUDP {
					res.Header["Transport"] = headers.Transport{
						Protocol: StreamProtocolUDP,
						Delivery: func() *base.StreamDelivery {
							v := base.StreamDeliveryUnicast
							return &v
						}(),
						ClientPorts: th.ClientPorts,
						ServerPorts: &[2]int{sc.s.conf.UDPRTPListener.port(), sc.s.conf.UDPRTCPListener.port()},
					}.Write()

					sc.tracks[trackID] = serverConnTrack{
						proto:    StreamProtocolUDP,
						rtpPort:  th.ClientPorts[0],
						rtcpPort: th.ClientPorts[1],
					}

				} else {
					res.Header["Transport"] = headers.Transport{
						Protocol:       StreamProtocolTCP,
						InterleavedIds: th.InterleavedIds,
					}.Write()

					sc.tracks[trackID] = serverConnTrack{
						proto: StreamProtocolTCP,
					}
				}
			}

			// workaround to prevent a bug in rtspclientsink
			// that makes impossible for the client to receive the response
			// and send frames.
			// this was causing problems during unit tests.
			if ua, ok := req.Header["User-Agent"]; ok && len(ua) == 1 &&
				strings.HasPrefix(ua[0], "GStreamer") {
				t := time.NewTimer(1 * time.Second)
				defer t.Stop()
				select {
				case <-t.C:
				case <-sc.terminate:
				}
			}

			return res, err
		}

	case base.Play:
		if sc.readHandlers.OnPlay != nil {
			res, err := sc.readHandlers.OnPlay(req)

			if res.StatusCode == 200 {
				sc.state = serverConnStatePlay
				sc.frameModeEnable()
			}

			return res, err
		}

	case base.Record:
		if sc.readHandlers.OnRecord != nil {
			res, err := sc.readHandlers.OnRecord(req)

			if res.StatusCode == 200 {
				sc.state = serverConnStateRecord
				sc.frameModeEnable()
			}

			return res, err
		}

	case base.Pause:
		if sc.readHandlers.OnPause != nil {
			res, err := sc.readHandlers.OnPause(req)

			if res.StatusCode == 200 {
				sc.frameModeDisable()
				sc.state = serverConnStateInitial
			}

			return res, err
		}

	case base.GetParameter:
		if sc.readHandlers.OnGetParameter != nil {
			return sc.readHandlers.OnGetParameter(req)
		}

		// GET_PARAMETER is used like a ping
		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"text/parameters"},
			},
			Content: []byte("\n"),
		}, nil

	case base.SetParameter:
		if sc.readHandlers.OnSetParameter != nil {
			return sc.readHandlers.OnSetParameter(req)
		}

	case base.Teardown:
		if sc.readHandlers.OnTeardown != nil {
			return sc.readHandlers.OnTeardown(req)
		}

		return &base.Response{
			StatusCode: base.StatusOK,
		}, ErrServerTeardown
	}

	return &base.Response{
		StatusCode: base.StatusBadRequest,
	}, fmt.Errorf("unhandled method: %v", req.Method)
}

func (sc *ServerConn) backgroundRead() error {
	handleRequestOuter := func(req *base.Request) error {
		// check cseq
		cseq, ok := req.Header["CSeq"]
		if !ok || len(cseq) != 1 {
			sc.writeMutex.Lock()
			sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
			base.Response{
				StatusCode: base.StatusBadRequest,
				Header:     base.Header{},
			}.Write(sc.bw)
			sc.writeMutex.Unlock()
			return errors.New("CSeq is missing")
		}

		res, err := sc.handleRequest(req)

		if res.Header == nil {
			res.Header = base.Header{}
		}

		// add cseq
		res.Header["CSeq"] = cseq

		// add server
		res.Header["Server"] = base.HeaderValue{"gortsplib"}

		if sc.readHandlers.OnResponse != nil {
			sc.readHandlers.OnResponse(res)
		}

		sc.writeMutex.Lock()

		sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
		res.Write(sc.bw)

		// set framesEnabled after sending the response
		// in order to start sending frames after the response, never before
		if sc.framesEnabled != sc.nextFramesEnabled {
			sc.framesEnabled = sc.nextFramesEnabled
		}

		sc.writeMutex.Unlock()

		return err
	}

	var req base.Request
	var frame base.InterleavedFrame
	tcpFrameBuffer := multibuffer.New(sc.s.conf.ReadBufferCount, clientTCPFrameReadBufferSize)
	var errRet error

outer:
	for {
		if sc.readTimeoutEnabled {
			sc.nconn.SetReadDeadline(time.Now().Add(sc.s.conf.ReadTimeout))
		} else {
			sc.nconn.SetReadDeadline(time.Time{})
		}

		if sc.framesEnabled {
			frame.Content = tcpFrameBuffer.Next()
			what, err := base.ReadInterleavedFrameOrRequest(&frame, &req, sc.br)
			if err != nil {
				errRet = err
				break outer
			}

			switch what.(type) {
			case *base.InterleavedFrame:
				sc.readHandlers.OnFrame(frame.TrackID, frame.StreamType, frame.Content)

			case *base.Request:
				err := handleRequestOuter(&req)
				if err != nil {
					errRet = err
					break outer
				}
			}

		} else {
			err := req.Read(sc.br)
			if err != nil {
				errRet = err
				break outer
			}

			err = handleRequestOuter(&req)
			if err != nil {
				errRet = err
				break outer
			}
		}
	}

	sc.frameModeDisable()

	return errRet
}

// Read starts reading requests and frames.
// it returns a channel that is written when the reading stops.
func (sc *ServerConn) Read(readHandlers ServerConnReadHandlers) chan error {
	// channel is buffered, since listening to it is not mandatory
	done := make(chan error, 1)

	sc.readHandlers = readHandlers

	go func() {
		done <- sc.backgroundRead()
	}()

	return done
}

// WriteFrame writes a frame.
func (sc *ServerConn) WriteFrame(trackID int, streamType StreamType, payload []byte) error {
	sc.writeMutex.Lock()
	defer sc.writeMutex.Unlock()

	track := sc.tracks[trackID]

	if track.proto == StreamProtocolUDP {
		if streamType == StreamTypeRtp {
			return sc.s.conf.UDPRTPListener.write(sc.s.conf.WriteTimeout, payload, &net.UDPAddr{
				IP:   sc.ip(),
				Zone: sc.zone(),
				Port: track.rtpPort,
			})
		}

		return sc.s.conf.UDPRTCPListener.write(sc.s.conf.WriteTimeout, payload, &net.UDPAddr{
			IP:   sc.ip(),
			Zone: sc.zone(),
			Port: track.rtcpPort,
		})
	}

	// StreamProtocolTCP

	if !sc.framesEnabled {
		return errors.New("frames are disabled")
	}

	sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Content:    payload,
	}
	return frame.Write(sc.bw)
}
