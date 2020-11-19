
# gortsplib

[![Build Status](https://travis-ci.com/aler9/gortsplib.svg?branch=master)](https://travis-ci.com/aler9/gortsplib)
[![Go Report Card](https://goreportcard.com/badge/github.com/aler9/gortsplib)](https://goreportcard.com/report/github.com/aler9/gortsplib)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aler9/gortsplib)](https://pkg.go.dev/github.com/aler9/gortsplib)

RTSP 1.0 client and server library for the Go programming language, written for [rtsp-simple-server](https://github.com/aler9/rtsp-simple-server).

Features:

* Client
  * Query servers about published streams
  * Read streams from servers with UDP or TCP
  * Publish streams to servers with UDP or TCP
  * Pause reading or publishing without disconnecting from the server
* Server
  * Handle server-side connections

## Examples

* [client-query](examples/client-query.go)
* [client-read-udp](examples/client-read-udp.go)
* [client-read-tcp](examples/client-read-tcp.go)
* [client-publish-udp](examples/client-publish-udp.go)
* [client-publish-tcp](examples/client-publish-tcp.go)
* [client-read-pause](examples/client-read-pause.go)

## Documentation

https://pkg.go.dev/github.com/aler9/gortsplib

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
