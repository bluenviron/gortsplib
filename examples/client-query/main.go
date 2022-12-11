package main

import (
	"log"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get and print informations about medias published on a path.

func main() {
	c := gortsplib.Client{}

	u, err := url.Parse("rtsp://localhost:8554/mypath")
	if err != nil {
		panic(err)
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	medias, _, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	log.Printf("available medias: %v\n", medias)
}
