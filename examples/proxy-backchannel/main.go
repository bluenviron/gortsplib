// Package main contains an example.
package main

import "log"

// This example shows how to
// 1. create a server that serves a single stream.
// 2. create a client, that reads an existing stream from another server or camera, containing a back channel.
// 3. route the stream from the client to the server, and from the server to all connected readers.
// 4. route the back channel from connected readers to the server, and from the server to the client.

func main() {
	// allocate the server.
	s := &server{}
	s.initialize()

	// allocate the client.
	// allow client to use the server.
	c := &client{server: s}
	c.initialize()

	// start server and wait until a fatal error
	log.Printf("server is ready on %s", s.server.RTSPAddress)
	panic(s.server.StartAndWait())
}
