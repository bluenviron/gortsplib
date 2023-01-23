package main

// This example shows how to
// 1. read an existing stream from an external server or camera, with a client
// 2. create a server that allow to proxy that stream

func main() {
	// allocate the client
	c := newClient()

	// allocate the server.
	// give server access to the method client.getStream().
	newServer(c.getStream)
}
