package gortsplib

import (
	"context"
	"net"
)

type clientConnCloser struct {
	ctx   context.Context
	nconn net.Conn

	terminate chan struct{}
	done      chan struct{}
}

func newClientConnCloser(ctx context.Context, nconn net.Conn) *clientConnCloser {
	cc := &clientConnCloser{
		ctx:       ctx,
		nconn:     nconn,
		terminate: make(chan struct{}),
		done:      make(chan struct{}),
	}

	go cc.run()

	return cc
}

func (cc *clientConnCloser) close() {
	close(cc.terminate)
	<-cc.done
}

func (cc *clientConnCloser) run() {
	defer close(cc.done)

	select {
	case <-cc.ctx.Done():
		cc.nconn.Close()

	case <-cc.terminate:
	}
}
