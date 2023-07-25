# gortsplib

[![Test](https://github.com/bluenviron/gortsplib/workflows/test/badge.svg)](https://github.com/bluenviron/gortsplib/actions?query=workflow:test)
[![Lint](https://github.com/bluenviron/gortsplib/workflows/lint/badge.svg)](https://github.com/bluenviron/gortsplib/actions?query=workflow:lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/bluenviron/gortsplib)](https://goreportcard.com/report/github.com/bluenviron/gortsplib)
[![CodeCov](https://codecov.io/gh/bluenviron/gortsplib/branch/main/graph/badge.svg)](https://app.codecov.io/gh/bluenviron/gortsplib/branch/main)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/bluenviron/gortsplib/v3)](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3#pkg-index)

RTSP 1.0 client and server library for the Go programming language, written for [MediaMTX](https://github.com/bluenviron/mediamtx).

Go &ge; 1.18 is required.

Features:

* Client
  * Query servers about available media streams
  * Read
    * Read media streams from servers with the UDP, UDP-multicast or TCP transport protocol
    * Read TLS-encrypted streams (TCP only)
    * Switch transport protocol automatically
    * Read only selected media streams
    * Pause or seek without disconnecting from the server
    * Generate RTCP receiver reports (UDP only)
    * Reorder incoming RTP packets (UDP only)
  * Publish
    * Publish media streams to servers with the UDP or TCP transport protocol
    * Publish TLS-encrypted streams (TCP only)
    * Switch transport protocol automatically
    * Pause without disconnecting from the server
    * Generate RTCP sender reports
* Server
  * Handle requests from clients
  * Sessions and connections are independent
  * Publish
    * Read media streams from clients with the UDP or TCP transport protocol
    * Read TLS-encrypted streams (TCP only)
    * Generate RTCP receiver reports (UDP only)
    * Reorder incoming RTP packets (UDP only)
  * Read
    * Write media streams to clients with the UDP, UDP-multicast or TCP transport protocol
    * Write TLS-encrypted streams
    * Compute and provide SSRC, RTP-Info to clients
    * Generate RTCP sender reports
* Utilities
  * Parse RTSP elements
  * Encode/decode format-specific frames into/from RTP packets

## Table of contents

* [Examples](#examples)
* [API Documentation](#api-documentation)
* [RTP Payload Formats](#rtp-payload-formats)
* [Standards](#standards)
* [Links](#links)

## Examples

* [client-query](examples/client-query/main.go)
* [client-read](examples/client-read/main.go)
* [client-read-options](examples/client-read-options/main.go)
* [client-read-pause](examples/client-read-pause/main.go)
* [client-read-republish](examples/client-read-republish/main.go)
* [client-read-format-g711](examples/client-read-format-g711/main.go)
* [client-read-format-g722](examples/client-read-format-g722/main.go)
* [client-read-format-h264](examples/client-read-format-h264/main.go)
* [client-read-format-h264-convert-to-jpeg](examples/client-read-format-h264-convert-to-jpeg/main.go)
* [client-read-format-h264-save-to-disk](examples/client-read-format-h264-save-to-disk/main.go)
* [client-read-format-h265](examples/client-read-format-h265/main.go)
* [client-read-format-lpcm](examples/client-read-format-lpcm/main.go)
* [client-read-format-mjpeg](examples/client-read-format-mjpeg/main.go)
* [client-read-format-mpeg4audio](examples/client-read-format-mpeg4audio/main.go)
* [client-read-format-mpeg4audio-save-to-disk](examples/client-read-format-mpeg4audio-save-to-disk/main.go)
* [client-read-format-opus](examples/client-read-format-opus/main.go)
* [client-read-format-vp8](examples/client-read-format-vp8/main.go)
* [client-read-format-vp9](examples/client-read-format-vp9/main.go)
* [client-publish-options](examples/client-publish-options/main.go)
* [client-publish-pause](examples/client-publish-pause/main.go)
* [client-publish-format-g711](examples/client-publish-format-g711/main.go)
* [client-publish-format-g722](examples/client-publish-format-g722/main.go)
* [client-publish-format-h264](examples/client-publish-format-h264/main.go)
* [client-publish-format-h265](examples/client-publish-format-h265/main.go)
* [client-publish-format-lpcm](examples/client-publish-format-lpcm/main.go)
* [client-publish-format-mjpeg](examples/client-publish-format-mjpeg/main.go)
* [client-publish-format-mpeg4audio](examples/client-publish-format-mpeg4audio/main.go)
* [client-publish-format-opus](examples/client-publish-format-opus/main.go)
* [client-publish-format-vp8](examples/client-publish-format-vp8/main.go)
* [client-publish-format-vp9](examples/client-publish-format-vp9/main.go)
* [server](examples/server/main.go)
* [server-tls](examples/server-tls/main.go)
* [server-h264-save-to-disk](examples/server-h264-save-to-disk/main.go)
* [proxy](examples/proxy/main.go)

## API Documentation

https://pkg.go.dev/github.com/bluenviron/gortsplib/v3#pkg-index

## RTP Payload Formats

In RTSP, media streams are routed between server and clients by using RTP packets. In order to decode a stream, RTP packets must be converted into codec-specific frames. This conversion happens by using a RTP payload format. This library recognizes the following formats:

### Video

|codec|rtp format|documentation|encoder and decoder available|
|-----|----------|-------------|-----------------------------|
|AV1|AV1|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#AV1)|:heavy_check_mark:|
|VP9|VP9|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#VP9)|:heavy_check_mark:|
|VP8|VP8|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#VP8)|:heavy_check_mark:|
|H265|H265|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#H265)|:heavy_check_mark:|
|H264|H264|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#H264)|:heavy_check_mark:|
|MPEG-2 Video|unnamed|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#MPEG2Video)||
|MPEG-4 Video (H263, Xvid)|MP4V-ES|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#MPEG4VideoES)|:heavy_check_mark:|
|M-JPEG|JPEG|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#MJPEG)|:heavy_check_mark:|

### Audio

|codec|rtp format|documentation|encoder and decoder available|
|-----|----------|-------------|-----------------------------|
|Opus|opus|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#Opus)|:heavy_check_mark:|
|Vorbis|VORBIS|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#Vorbis)||
|MPEG-4 Audio (AAC)|mpeg4-generic|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#MPEG4AudioGeneric)|:heavy_check_mark:|
|MPEG-4 Audio (AAC)|MP4A-LATM|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#MPEG4AudioLATM)|:heavy_check_mark:|
|MPEG-1/2 Audio (MP3)|unnamed|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#MPEG2Audio)|:heavy_check_mark:|
|G722|unnamed|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#G722)|:heavy_check_mark:|
|G711 (PCMA, PCMU)|unnamed|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#G711)|:heavy_check_mark:|
|LPCM|L8/L16/L24|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#LPCM)|:heavy_check_mark:|

### Mixed

|codec|rtp format|documentation|encoder and decoder available|
|-----|----------|-------------|-----------------------------|
|MPEG-TS|MP2T|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v3/pkg/formats#MPEGTS)||

## Standards

* [RFC2326, RTSP 1.0](https://datatracker.ietf.org/doc/html/rfc2326)
* [RFC7826, RTSP 2.0](https://datatracker.ietf.org/doc/html/rfc7826)
* [RFC8866, SDP: Session Description Protocol](https://datatracker.ietf.org/doc/html/rfc8866)
* [RFC3551, RTP Profile for Audio and Video Conferences with Minimal Control](https://datatracker.ietf.org/doc/html/rfc3551)
* [RFC2250, RTP Payload Format for MPEG1/MPEG2 Video](https://datatracker.ietf.org/doc/html/rfc2250)
* [RFC2435, RTP Payload Format for JPEG-compressed Video](https://datatracker.ietf.org/doc/html/rfc2435)
* [RFC6416, RTP Payload Format for MPEG-4 Audio/Visual Streams](https://datatracker.ietf.org/doc/html/rfc6416)
* [RFC6184, RTP Payload Format for H.264 Video](https://datatracker.ietf.org/doc/html/rfc6184)
* [RFC7798, RTP Payload Format for High Efficiency Video Coding (HEVC)](https://datatracker.ietf.org/doc/html/rfc7798)
* [RFC7741, RTP Payload Format for VP8 Video](https://datatracker.ietf.org/doc/html/rfc7741)
* [RTP Payload Format for VP9 Video](https://datatracker.ietf.org/doc/html/draft-ietf-payload-vp9-16)
* [RFC3190, RTP Payload Format for 12-bit DAT Audio and 20- and 24-bit Linear Sampled Audio](https://datatracker.ietf.org/doc/html/rfc3190)
* [RFC5215, RTP Payload Format for Vorbis Encoded Audio](https://datatracker.ietf.org/doc/html/rfc5215)
* [RFC7587, RTP Payload Format for the Opus Speech and Audio Codec](https://datatracker.ietf.org/doc/html/rfc7587)
* [RFC3640, RTP Payload Format for Transport of MPEG-4 Elementary Streams](https://datatracker.ietf.org/doc/html/rfc3640)
* [RTP Payload Format For AV1 (v1.0)](https://aomediacodec.github.io/av1-rtp-spec/)
* [Codec standards](https://github.com/bluenviron/mediacommon#standards)
* [Golang project layout](https://github.com/golang-standards/project-layout)

## Links

Related projects

* [MediaMTX](https://github.com/bluenviron/mediamtx)
* [pion/sdp (SDP library used internally)](https://github.com/pion/sdp)
* [pion/rtp (RTP library used internally)](https://github.com/pion/rtp)
* [pion/rtcp (RTCP library used internally)](https://github.com/pion/rtcp)
