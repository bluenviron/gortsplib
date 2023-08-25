package main

import "log"

// This example shows how to
// 1. create a server that serves a single stream.
// 2. create a client, read an existing stream from an external server or camera,
//    pass the stream to the server in order to serve it.

func main() {
	// allocate the server.
	s := newServer()

	// allocate the client.
	// give client access to the server.
	newClient(s)

	// start server and wait until a fatal error
	log.Printf("server is ready")
	s.s.StartAndWait()
}
