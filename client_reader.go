package gortsplib

import (
	"time"

	"github.com/bluenviron/gortsplib/v3/pkg/base"
)

type clientReader struct {
	c        *Client
	closeErr chan error
}

func newClientReader(c *Client) *clientReader {
	r := &clientReader{
		c:        c,
		closeErr: make(chan error),
	}

	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we disable the deadline and perform a check with a ticker.
	r.c.nconn.SetReadDeadline(time.Time{})

	go r.run()

	return r
}

func (r *clientReader) close() {
	r.c.nconn.SetReadDeadline(time.Now())
}

func (r *clientReader) run() {
	r.c.readError <- r.runInner()
}

func (r *clientReader) runInner() error {
	if *r.c.effectiveTransport == TransportUDP || *r.c.effectiveTransport == TransportUDPMulticast {
		for {
			res, err := r.c.conn.ReadResponse()
			if err != nil {
				return err
			}

			r.c.OnResponse(res)
		}
	} else {
		for {
			what, err := r.c.conn.ReadInterleavedFrameOrResponse()
			if err != nil {
				return err
			}

			switch what := what.(type) {
			case *base.Response:
				r.c.OnResponse(what)

			case *base.InterleavedFrame:
				channel := what.Channel
				isRTP := true
				if (channel % 2) != 0 {
					channel--
					isRTP = false
				}

				media, ok := r.c.tcpMediasByChannel[channel]
				if !ok {
					continue
				}

				if isRTP {
					err = media.readRTP(what.Payload)
				} else {
					err = media.readRTCP(what.Payload)
				}
				if err != nil {
					return err
				}
			}
		}
	}
}
