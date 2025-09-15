// Package main contains an example.
package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

// This example shows how to:
// 1. connect to a RTSP server.
// 2. get and print informations about medias published on a path.

func main() {
	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/mypath")
	if err != nil {
		panic(err)
	}

	c := gortsplib.Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	err = c.Start2()
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
