
# gortsplib

[![Build Status](https://travis-ci.com/aler9/gortsplib.svg?branch=master)](https://travis-ci.com/aler9/gortsplib)
[![Go Report Card](https://goreportcard.com/badge/github.com/aler9/gortsplib)](https://goreportcard.com/report/github.com/aler9/gortsplib)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aler9/gortsplib)](https://pkg.go.dev/github.com/aler9/gortsplib)

RTSP 1.0 client and server library for the Go programming language, written for [rtsp-simple-server](https://github.com/aler9/rtsp-simple-server).

Features:
* Read streams with TCP or UDP
* Publish streams with TCP or UDP
* Provides primitives
* Provides a class for building clients (`ConnClient`)
* Provides a class for building servers (`ConnServer`)

## Examples

* [client-read-tcp](examples/client-read-tcp.go)
* [client-read-udp](examples/client-read-udp.go)
* [client-publish-tcp](examples/client-publish-tcp.go)
* [client-publish-udp](examples/client-publish-udp.go)

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
