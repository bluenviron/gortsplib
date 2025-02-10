package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get and print informations about medias published on a path.

func main() {
	c := gortsplib.Client{}

	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/mypath")
	if err != nil {
		panic(err)
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	log.Printf("available medias: %v\n", desc.Medias)
}
