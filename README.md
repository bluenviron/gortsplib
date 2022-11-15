
# gortsplib

[![Test](https://github.com/aler9/gortsplib/workflows/test/badge.svg)](https://github.com/aler9/gortsplib/actions?query=workflow:test)
[![Lint](https://github.com/aler9/gortsplib/workflows/lint/badge.svg)](https://github.com/aler9/gortsplib/actions?query=workflow:lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/aler9/gortsplib)](https://goreportcard.com/report/github.com/aler9/gortsplib)
[![CodeCov](https://codecov.io/gh/aler9/gortsplib/branch/main/graph/badge.svg)](https://codecov.io/gh/aler9/gortsplib/branch/main)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aler9/gortsplib)](https://pkg.go.dev/github.com/aler9/gortsplib#pkg-index)

RTSP 1.0 client and server library for the Go programming language, written for [rtsp-simple-server](https://github.com/aler9/rtsp-simple-server).

Go &ge; 1.17 is required.

Features:

* Client
  * Query servers about available streams and tracks
  * Read
    * Read streams from servers with the UDP, UDP-multicast or TCP transport protocol
    * Read TLS-encrypted streams (TCP only)
    * Switch transport protocol automatically
    * Read only selected tracks of a stream
    * Pause or seek without disconnecting from the server
    * Generate RTCP receiver reports (UDP only)
    * Reorder incoming RTP packets (UDP only)
  * Publish
    * Publish streams to servers with the UDP or TCP transport protocol
    * Publish TLS-encrypted streams (TCP only)
    * Switch transport protocol automatically
    * Pause without disconnecting from the server
    * Generate RTCP sender reports
* Server
  * Handle requests from clients
  * Sessions and connections are independent
  * Publish
    * Read streams from clients with the UDP or TCP transport protocol
    * Read TLS-encrypted streams (TCP only)
    * Generate RTCP receiver reports (UDP only)
    * Reorder incoming RTP packets (UDP only)
  * Read
    * Write streams to clients with the UDP, UDP-multicast or TCP transport protocol
    * Write TLS-encrypted streams
    * Compute and provide SSRC, RTP-Info to clients
    * Generate RTCP sender reports
* Utilities
  * Encode and decode codec-specific frames into/from RTP packets. The following codecs are supported:
    * Video
      * H264
      * H265
      * VP8
      * VP9
    * Audio
      * G722
      * LPCM
      * MPEG4-audio (AAC)
      * Opus
      * PCMA
      * PCMU
  * Parse RTSP elements: requests, responses, SDP
  * Parse H264 elements and formats: Annex-B, AVCC, anti-competition, DTS
  * Parse MPEG4-audio (AAC) element and formats: ADTS, MPEG4-audio configurations

## Table of contents

* [Examples](#examples)
* [API Documentation](#api-documentation)
* [Links](#links)

## Examples

* [client-query](examples/client-query/main.go)
* [client-read](examples/client-read/main.go)
* [client-read-codec-g722](examples/client-read-codec-g722/main.go)
* [client-read-codec-h264](examples/client-read-codec-h264/main.go)
* [client-read-codec-h264-convert-to-jpeg](examples/client-read-codec-h264-convert-to-jpeg/main.go)
* [client-read-codec-h264-save-to-disk](examples/client-read-codec-h264-save-to-disk/main.go)
* [client-read-codec-h265](examples/client-read-codec-h265/main.go)
* [client-read-codec-lpcm](examples/client-read-codec-lpcm/main.go)
* [client-read-codec-mpeg4audio](examples/client-read-codec-mpeg4audio/main.go)
* [client-read-codec-opus](examples/client-read-codec-opus/main.go)
* [client-read-codec-pcma](examples/client-read-codec-pcma/main.go)
* [client-read-codec-pcmu](examples/client-read-codec-pcmu/main.go)
* [client-read-codec-vp8](examples/client-read-codec-vp8/main.go)
* [client-read-codec-vp9](examples/client-read-codec-vp9/main.go)
* [client-read-partial](examples/client-read-partial/main.go)
* [client-read-options](examples/client-read-options/main.go)
* [client-read-pause](examples/client-read-pause/main.go)
* [client-read-republish](examples/client-read-republish/main.go)
* [client-publish-codec-g722](examples/client-publish-codec-g722/main.go)
* [client-publish-codec-h264](examples/client-publish-codec-h264/main.go)
* [client-publish-codec-h265](examples/client-publish-codec-h265/main.go)
* [client-publish-codec-lpcm](examples/client-publish-codec-lpcm/main.go)
* [client-publish-codec-mpeg4audio](examples/client-publish-codec-mpeg4audio/main.go)
* [client-publish-codec-opus](examples/client-publish-codec-opus/main.go)
* [client-publish-codec-pcma](examples/client-publish-codec-pcma/main.go)
* [client-publish-codec-pcmu](examples/client-publish-codec-pcmu/main.go)
* [client-publish-codec-vp8](examples/client-publish-codec-vp8/main.go)
* [client-publish-codec-vp9](examples/client-publish-codec-vp9/main.go)
* [client-publish-options](examples/client-publish-options/main.go)
* [client-publish-pause](examples/client-publish-pause/main.go)
* [server](examples/server/main.go)
* [server-tls](examples/server-tls/main.go)
* [server-h264-save-to-disk](examples/server-h264-save-to-disk/main.go)

## API Documentation

https://pkg.go.dev/github.com/aler9/gortsplib#pkg-index

## Links

Related projects

* rtsp-simple-server https://github.com/aler9/rtsp-simple-server
* pion/sdp (SDP library used internally) https://github.com/pion/sdp
* pion/rtp (RTP library used internally) https://github.com/pion/rtp
* pion/rtcp (RTCP library used internally) https://github.com/pion/rtcp

Standards

* RTSP 1.0 https://tools.ietf.org/html/rfc2326
* RTSP 2.0 https://tools.ietf.org/html/rfc7826
* HTTP 1.1 https://tools.ietf.org/html/rfc2616
* Golang project layout https://github.com/golang-standards/project-layout
