// +build ignore

package main

import (
	"fmt"
	"strings"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

func handleConn(conn *gortsplib.ServerConn) {
	defer conn.Close()

	onRequest := func(req *base.Request) (*base.Response, error) {
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

	onFrame := func(id int, typ gortsplib.StreamType, buf []byte) {
	}

	done := conn.Read(onRequest, onFrame)

	err := <-done
	panic(err)
}

func main() {
	// create server
	s, err := gortsplib.Serve(":8554")
	if err != nil {
		panic(err)
	}

	// accept connections
	for {
		conn, err := s.Accept()
		if err != nil {
			panic(err)
		}

		go handleConn(conn)
	}
}
