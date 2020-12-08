// +build ignore

package main

import (
	"fmt"
	"strings"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

type serverConnHandler struct {
}

func (sc *serverConnHandler) OnClose(err error) {
}

func (sc *serverConnHandler) OnRequest(req *base.Request) (*base.Response, error) {
	switch req.Method {
	case base.Options:
		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
					string(base.Announce),
					string(base.Setup),
					string(base.Play),
					string(base.Record),
					string(base.Pause),
					string(base.Teardown),
				}, ", ")},
			},
		}, nil

	case base.Teardown:
		return &base.Response{
			StatusCode: base.StatusOK,
			Header:     base.Header{},
		}, fmt.Errorf("terminated")
	}

	return &base.Response{
		StatusCode: base.StatusBadRequest,
		Header:     base.Header{},
	}, fmt.Errorf("unhandled method: %v", req.Method)
}

func (sc *serverConnHandler) OnFrame(id int, typ gortsplib.StreamType, buf []byte) {
}

func main() {
	// create server
	gortsplib.Serve(":8554", func(c *gortsplib.ServerConn) gortsplib.ServerConnHandler {
		return &serverConnHandler{}
	})

	// wait forever
	select {}
}
