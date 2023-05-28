package gortsplib

import (
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v3/pkg/base"
	"github.com/bluenviron/gortsplib/v3/pkg/liberrors"
)

type errSwitchReadFunc struct {
	tcp bool
}

func (errSwitchReadFunc) Error() string {
	return "switching read function"
}

func isErrSwitchReadFunc(err error) bool {
	_, ok := err.(errSwitchReadFunc)
	return ok
}

type serverConnReader struct {
	sc *ServerConn

	chReadDone chan struct{}
}

func newServerConnReader(sc *ServerConn) *serverConnReader {
	cr := &serverConnReader{
		sc:         sc,
		chReadDone: make(chan struct{}),
	}

	go cr.run()

	return cr
}

func (cr *serverConnReader) wait() {
	<-cr.chReadDone
}

func (cr *serverConnReader) run() {
	defer close(cr.chReadDone)

	readFunc := cr.readFuncStandard

	for {
		err := readFunc()
		if err, ok := err.(errSwitchReadFunc); ok {
			if err.tcp {
				readFunc = cr.readFuncTCP
			} else {
				readFunc = cr.readFuncStandard
			}
			continue
		}

		cr.sc.readErr(err)
		break
	}
}

func (cr *serverConnReader) readFuncStandard() error {
	// reset deadline
	cr.sc.nconn.SetReadDeadline(time.Time{})

	for {
		what, err := cr.sc.conn.ReadInterleavedFrameOrRequest()
		if err != nil {
			return err
		}

		switch what := what.(type) {
		case *base.Request:
			cres := make(chan error)
			req := readReq{req: what, res: cres}
			err := cr.sc.handleRequest(req)
			if err != nil {
				return err
			}

		default:
			return liberrors.ErrServerUnexpectedFrame{}
		}
	}
}

func (cr *serverConnReader) readFuncTCP() error {
	// reset deadline
	cr.sc.nconn.SetReadDeadline(time.Time{})

	cr.sc.session.startWriter()

	for {
		if cr.sc.session.state == ServerSessionStateRecord {
			cr.sc.nconn.SetReadDeadline(time.Now().Add(cr.sc.s.ReadTimeout))
		}

		what, err := cr.sc.conn.ReadInterleavedFrameOrRequest()
		if err != nil {
			return err
		}

		switch twhat := what.(type) {
		case *base.InterleavedFrame:
			channel := twhat.Channel
			isRTP := true
			if (channel % 2) != 0 {
				channel--
				isRTP = false
			}

			atomic.AddUint64(cr.sc.session.bytesReceived, uint64(len(twhat.Payload)))

			if sm, ok := cr.sc.session.tcpMediasByChannel[channel]; ok {
				if isRTP {
					sm.readRTP(twhat.Payload)
				} else {
					sm.readRTCP(twhat.Payload)
				}
			}

		case *base.Request:
			cres := make(chan error)
			req := readReq{req: twhat, res: cres}
			err := cr.sc.handleRequest(req)
			if err != nil {
				return err
			}
		}
	}
}
