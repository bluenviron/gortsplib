package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get and print informations about tracks published on a path.

func main() {
	c := gortsplib.Client{}

	u, err := base.ParseURL("rtsp://localhost:8554/mypath")
	if err != nil {
		panic(err)
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	tracks, _, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	log.Println("available tracks: %v\n", tracks)
}
