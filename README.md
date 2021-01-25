
# gortsplib

[![Test](https://github.com/aler9/gortsplib/workflows/test/badge.svg)](https://github.com/aler9/gortsplib/actions)
[![Lint](https://github.com/aler9/gortsplib/workflows/lint/badge.svg)](https://github.com/aler9/gortsplib/actions)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aler9/gortsplib)](https://pkg.go.dev/github.com/aler9/gortsplib#pkg-index)

RTSP 1.0 client and server library for the Go programming language, written for [rtsp-simple-server](https://github.com/aler9/rtsp-simple-server).

Features:

* Client
  * Read streams from servers with UDP or TCP
  * Publish streams to servers with UDP or TCP
  * Encrypt streams with TLS (RTSPS)
  * Query servers about published streams
  * Read only selected tracks of a stream
  * Pause reading or publishing without disconnecting from the server
* Server
  * Handle requests from clients
  * Accept streams from clients with UDP or TCP
  * Send streams to clients with UDP or TCP
  * Encrypt streams with TLS (RTSPS)

## Table of contents

* [Examples](#examples)
* [API Documentation](#api-documentation)
* [Links](#links)

## Examples

* [client-query](examples/client-query.go)
* [client-read](examples/client-read.go)
* [client-read-partial](examples/client-read-partial.go)
* [client-read-options](examples/client-read-options.go)
* [client-read-pause](examples/client-read-pause.go)
* [client-publish](examples/client-publish.go)
* [client-publish-options](examples/client-publish-options.go)
* [client-publish-pause](examples/client-publish-pause.go)
* [server](examples/server.go)
* [server-udp](examples/server-udp.go)
* [server-tls](examples/server-tls.go)

## API Documentation

https://pkg.go.dev/github.com/aler9/gortsplib#pkg-index

## Links

Related projects

* https://github.com/aler9/rtsp-simple-server
* https://github.com/pion/sdp (SDP library used internally)
* https://github.com/pion/rtcp (RTCP library used internally)
* https://github.com/pion/rtp (RTP library used internally)
* https://github.com/notedit/rtmp (RTMP library used internally)

IETF Standards

* RTSP 1.0 https://tools.ietf.org/html/rfc2326
* RTSP 2.0 https://tools.ietf.org/html/rfc7826
* HTTP 1.1 https://tools.ietf.org/html/rfc2616

Conventions

* https://github.com/golang-standards/project-layout
