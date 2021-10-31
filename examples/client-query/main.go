package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get and print informations about tracks published on a path.

func main() {
	u, err := base.ParseURL("rtsp://myserver/mypath")
	if err != nil {
		panic(err)
	}

	c := gortsplib.Client{}

	err = c.Dial(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	_, err = c.Options(u)
	if err != nil {
		panic(err)
	}

	tracks, _, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	fmt.Println("available tracks: %v\n", tracks)
}
