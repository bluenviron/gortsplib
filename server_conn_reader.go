package gortsplib

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
	"github.com/bluenviron/mediacommon/v2/pkg/rewindablereader"
)

func makeReadWriter(r io.Reader, w io.Writer) io.ReadWriter {
	return struct {
		io.Reader
		io.Writer
	}{r, w}
}

type switchReadFuncError struct {
	tcp bool
}

func (switchReadFuncError) Error() string {
	return "switching read function"
}

func isSwitchReadFuncError(err error) bool {
	var eerr switchReadFuncError
	return errors.As(err, &eerr)
}

type serverConnReader struct {
	sc *ServerConn

	done chan struct{}
}

func (cr *serverConnReader) initialize() {
	cr.done = make(chan struct{})

	go cr.run()
}

func (cr *serverConnReader) wait() {
	<-cr.done
}

func (cr *serverConnReader) run() {
	defer close(cr.done)

	err := cr.runInner()

	select {
	case cr.sc.chReadError <- err:
	case <-cr.sc.ctx.Done():
	}
}

func (cr *serverConnReader) runInner() error {
	var rw io.ReadWriter = cr.sc.bc

	if !cr.sc.isHTTP {
		var err error
		rw, err = cr.upgradeToHTTP(rw)
		if err != nil {
			return err
		}
	}

	cr.sc.conn = conn.NewConn(bufio.NewReader(rw), rw)

	readFunc := cr.readFuncStandard

	for {
		err := readFunc()

		var eerr switchReadFuncError
		if errors.As(err, &eerr) {
			if eerr.tcp {
				readFunc = cr.readFuncTCP
			} else {
				readFunc = cr.readFuncStandard
			}
			continue
		}

		return err
	}
}

func (cr *serverConnReader) upgradeToHTTP(in io.ReadWriter) (io.ReadWriter, error) {
	rr := &rewindablereader.Reader{R: in}

	buf := make([]byte, 4)
	_, err := io.ReadFull(rr, buf)
	if err != nil {
		return nil, err
	}

	rr.Rewind()

	if bytes.Equal(buf, []byte{'G', 'E', 'T', ' '}) ||
		bytes.Equal(buf, []byte{'P', 'O', 'S', 'T'}) {
		buf := bufio.NewReader(rr)
		var req *http.Request
		req, err = http.ReadRequest(buf)
		if err != nil {
			return nil, err
		}

		if (req.Method != http.MethodGet && req.Method != http.MethodPost) ||
			(req.Method == http.MethodGet && req.Header.Get("Accept") != "application/x-rtsp-tunnelled") ||
			(req.Method == http.MethodPost && req.Header.Get("Content-Type") != "application/x-rtsp-tunnelled") ||
			req.Header.Get("X-Sessioncookie") == "" {
			res := http.Response{
				StatusCode: http.StatusBadRequest,
				ProtoMajor: req.ProtoMajor,
				ProtoMinor: req.ProtoMinor,
				Request:    req,
			}
			var buf2 bytes.Buffer
			res.Write(&buf2) //nolint:errcheck
			cr.sc.nconn.SetWriteDeadline(time.Now().Add(cr.sc.s.WriteTimeout))
			_, err = in.Write(buf2.Bytes())
			if err != nil {
				return nil, err
			}

			return nil, fmt.Errorf("invalid HTTP request")
		}

		h := http.Header{}
		h.Set("Cache-Control", "no-cache")
		h.Set("Connection", "close")
		h.Set("Content-Type", "application/x-rtsp-tunnelled")
		h.Set("Pragma", "no-cache")
		res := http.Response{
			StatusCode:    http.StatusOK,
			ProtoMajor:    1,
			ProtoMinor:    req.ProtoMinor,
			Header:        h,
			ContentLength: -1,
		}
		var buf2 bytes.Buffer
		res.Write(&buf2) //nolint:errcheck
		cr.sc.nconn.SetWriteDeadline(time.Now().Add(cr.sc.s.WriteTimeout))
		_, err = in.Write(buf2.Bytes())
		if err != nil {
			return nil, err
		}

		cr.sc.httpReadBuf = buf

		err = cr.sc.s.handleHTTPChannel(sessionHandleHTTPChannelReq{
			sc:       cr.sc,
			write:    (req.Method == http.MethodPost),
			tunnelID: req.Header.Get("X-Sessioncookie"),
		})
		return nil, err
	}

	return makeReadWriter(rr, in), nil
}

func (cr *serverConnReader) readFuncStandard() error {
	// reset deadline
	cr.sc.nconn.SetReadDeadline(time.Time{})

	for {
		what, err := cr.sc.conn.Read()
		if err != nil {
			return err
		}

		switch what := what.(type) {
		case *base.Request:
			cres := make(chan error)
			req := readReq{req: what, res: cres}

			select {
			case cr.sc.chRequest <- req:
			case <-cr.sc.ctx.Done():
				return fmt.Errorf("terminated")
			}

			err = <-cres
			if err != nil {
				return err
			}

		case *base.Response:
			return liberrors.ErrServerUnexpectedResponse{}

		case *base.InterleavedFrame:
			return liberrors.ErrServerUnexpectedFrame{}
		}
	}
}

func (cr *serverConnReader) readFuncTCP() error {
	// reset deadline
	cr.sc.nconn.SetReadDeadline(time.Time{})

	cr.sc.session.asyncStartWriter()

	for {
		if cr.sc.session.state == ServerSessionStateRecord {
			cr.sc.nconn.SetReadDeadline(time.Now().Add(cr.sc.s.ReadTimeout))
		}

		what, err := cr.sc.conn.Read()
		if err != nil {
			return err
		}

		switch what := what.(type) {
		case *base.Request:
			cres := make(chan error)
			req := readReq{req: what, res: cres}

			select {
			case cr.sc.chRequest <- req:
			case <-cr.sc.ctx.Done():
				return fmt.Errorf("terminated")
			}

			err = <-cres
			if err != nil {
				return err
			}

		case *base.Response:
			return liberrors.ErrServerUnexpectedResponse{}

		case *base.InterleavedFrame:
			if cb, ok := cr.sc.session.tcpCallbackByChannel[what.Channel]; ok {
				cb(what.Payload)
			}
		}
	}
}
