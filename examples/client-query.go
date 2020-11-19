// +build ignore

package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

// This example shows how to connect to a server and print informations about
// tracks published on a path.

func main() {
	u, err := base.ParseURL("rtsp://myserver/mypath")
	if err != nil {
		panic(err)
	}

	conn, err := gortsplib.Dial(u.Host)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Options(u)
	if err != nil {
		panic(err)
	}

	tracks, res, err := conn.Describe(u)
	if err != nil {
		panic(err)
	}

	if res.StatusCode != base.StatusOK {
		panic(fmt.Errorf("server returned status %d", res.StatusCode))
	}

	fmt.Println("available tracks: %v\n", tracks)
}
