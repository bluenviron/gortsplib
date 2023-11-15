# gortsplib

[![Test](https://github.com/bluenviron/gortsplib/workflows/test/badge.svg)](https://github.com/bluenviron/gortsplib/actions?query=workflow:test)
[![Lint](https://github.com/bluenviron/gortsplib/workflows/lint/badge.svg)](https://github.com/bluenviron/gortsplib/actions?query=workflow:lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/bluenviron/gortsplib)](https://goreportcard.com/report/github.com/bluenviron/gortsplib)
[![CodeCov](https://codecov.io/gh/bluenviron/gortsplib/branch/main/graph/badge.svg)](https://app.codecov.io/gh/bluenviron/gortsplib/branch/main)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/bluenviron/gortsplib/v4)](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4#pkg-index)

RTSP 1.0 client and server library for the Go programming language, written for [MediaMTX](https://github.com/bluenviron/mediamtx).

Go &ge; 1.19 is required.

Features:

* Client
  * Query servers about available media streams
  * Play (read)
    * Read media streams from servers with the UDP, UDP-multicast or TCP transport protocol
    * Read TLS-encrypted streams (TCP only)
    * Switch transport protocol automatically
    * Read selected media streams
    * Pause or seek without disconnecting from the server
    * Write to ONVIF back channels
    * Get PTS (relative) timestamp of incoming packets
    * Get NTP (absolute) timestamp of incoming packets
  * Record (write)
    * Write media streams to servers with the UDP or TCP transport protocol
    * Write TLS-encrypted streams (TCP only)
    * Switch transport protocol automatically
    * Pause without disconnecting from the server
* Server
  * Handle requests from clients
  * Record (read)
    * Read media streams from clients with the UDP or TCP transport protocol
    * Read TLS-encrypted streams (TCP only)
    * Get PTS (relative) timestamp of incoming packets
    * Get NTP (absolute) timestamp of incoming packets
  * Play (write)
    * Write media streams to clients with the UDP, UDP-multicast or TCP transport protocol
    * Write TLS-encrypted streams (TCP only)
    * Compute and provide SSRC, RTP-Info to clients
* Utilities
  * Parse RTSP elements
  * Encode/decode RTP packets into/from codec-specific frames

## Table of contents

* [Examples](#examples)
* [API Documentation](#api-documentation)
* [RTP Payload Formats](#rtp-payload-formats)
* [Specifications](#specifications)
* [Related projects](#related-projects)

## Examples

* [client-query](examples/client-query/main.go)
* [client-play](examples/client-play/main.go)
* [client-play-timestamp](examples/client-play-timestamp/main.go)
* [client-play-options](examples/client-play-options/main.go)
* [client-play-pause](examples/client-play-pause/main.go)
* [client-play-to-record](examples/client-play-to-record/main.go)
* [client-play-backchannel](examples/client-play-backchannel/main.go)
* [client-play-format-av1](examples/client-play-format-av1/main.go)
* [client-play-format-g711](examples/client-play-format-g711/main.go)
* [client-play-format-g722](examples/client-play-format-g722/main.go)
* [client-play-format-h264](examples/client-play-format-h264/main.go)
* [client-play-format-h264-convert-to-jpeg](examples/client-play-format-h264-convert-to-jpeg/main.go)
* [client-play-format-h264-save-to-disk](examples/client-play-format-h264-save-to-disk/main.go)
* [client-play-format-h265](examples/client-play-format-h265/main.go)
* [client-play-format-h265-convert-to-jpeg](examples/client-play-format-h265-convert-to-jpeg/main.go)
* [client-play-format-h265-save-to-disk](examples/client-play-format-h265-save-to-disk/main.go)
* [client-play-format-lpcm](examples/client-play-format-lpcm/main.go)
* [client-play-format-mjpeg](examples/client-play-format-mjpeg/main.go)
* [client-play-format-mpeg4audio](examples/client-play-format-mpeg4audio/main.go)
* [client-play-format-mpeg4audio-save-to-disk](examples/client-play-format-mpeg4audio-save-to-disk/main.go)
* [client-play-format-opus](examples/client-play-format-opus/main.go)
* [client-play-format-vp8](examples/client-play-format-vp8/main.go)
* [client-play-format-vp9](examples/client-play-format-vp9/main.go)
* [client-record-options](examples/client-record-options/main.go)
* [client-record-pause](examples/client-record-pause/main.go)
* [client-record-format-g711](examples/client-record-format-g711/main.go)
* [client-record-format-g722](examples/client-record-format-g722/main.go)
* [client-record-format-h264](examples/client-record-format-h264/main.go)
* [client-record-format-h265](examples/client-record-format-h265/main.go)
* [client-record-format-lpcm](examples/client-record-format-lpcm/main.go)
* [client-record-format-mjpeg](examples/client-record-format-mjpeg/main.go)
* [client-record-format-mpeg4audio](examples/client-record-format-mpeg4audio/main.go)
* [client-record-format-opus](examples/client-record-format-opus/main.go)
* [client-record-format-vp8](examples/client-record-format-vp8/main.go)
* [client-record-format-vp9](examples/client-record-format-vp9/main.go)
* [server](examples/server/main.go)
* [server-tls](examples/server-tls/main.go)
* [server-h264-save-to-disk](examples/server-h264-save-to-disk/main.go)
* [proxy](examples/proxy/main.go)

## API Documentation

[Click to open the API Documentation](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4#pkg-index)

## RTP Payload Formats

In RTSP, media streams are routed between server and clients by using RTP packets, which are encoded in a specific, codec-dependent, format. This library supports the following formats:

### Video

|format|documentation|encoder and decoder available|
|------|-------------|-----------------------------|
|AV1|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#AV1)|:heavy_check_mark:|
|VP9|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#VP9)|:heavy_check_mark:|
|VP8|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#VP8)|:heavy_check_mark:|
|H265|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#H265)|:heavy_check_mark:|
|H264|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#H264)|:heavy_check_mark:|
|MPEG-4 Video (H263, Xvid)|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#MPEG4Video)|:heavy_check_mark:|
|MPEG-1/2 Video|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#MPEG1Video)|:heavy_check_mark:|
|M-JPEG|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#MJPEG)|:heavy_check_mark:|

### Audio

|format|documentation|encoder and decoder available|
|------|-------------|-----------------------------|
|Opus|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#Opus)|:heavy_check_mark:|
|Vorbis|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#Vorbis)||
|MPEG-4 Audio (AAC)|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#MPEG4Audio)|:heavy_check_mark:|
|MPEG-1/2 Audio (MP3)|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#MPEG1Audio)|:heavy_check_mark:|
|AC-3|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#AC3)|:heavy_check_mark:|
|Speex|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#Speex)||
|G726|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#G726)||
|G722|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#G722)|:heavy_check_mark:|
|G711 (PCMA, PCMU)|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#G711)|:heavy_check_mark:|
|LPCM|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#LPCM)|:heavy_check_mark:|

### Other

|format|documentation|encoder and decoder available|
|------|-------------|-----------------------------|
|MPEG-TS|[link](https://pkg.go.dev/github.com/bluenviron/gortsplib/v4/pkg/format#MPEGTS)||

## Specifications

|name|area|
|----|----|
|[RFC2326, RTSP 1.0](https://datatracker.ietf.org/doc/html/rfc2326)|protocol|
|[RFC7826, RTSP 2.0](https://datatracker.ietf.org/doc/html/rfc7826)|protocol|
|[RFC8866, SDP: Session Description Protocol](https://datatracker.ietf.org/doc/html/rfc8866)|SDP|
|[RTP Payload Format For AV1 (v1.0)](https://aomediacodec.github.io/av1-rtp-spec/)|AV1 payload format|
|[RTP Payload Format for VP9 Video](https://datatracker.ietf.org/doc/html/draft-ietf-payload-vp9-16)|VP9 payload format|
|[RFC7741, RTP Payload Format for VP8 Video](https://datatracker.ietf.org/doc/html/rfc7741)|VP8 payload format|
|[RFC7798, RTP Payload Format for High Efficiency Video Coding (HEVC)](https://datatracker.ietf.org/doc/html/rfc7798)|H265 payload format|
|[RFC6184, RTP Payload Format for H.264 Video](https://datatracker.ietf.org/doc/html/rfc6184)|H264 payload format|
|[RFC3640, RTP Payload Format for Transport of MPEG-4 Elementary Streams](https://datatracker.ietf.org/doc/html/rfc3640)|MPEG-4 audio, MPEG-4 video payload formats|
|[RFC2250, RTP Payload Format for MPEG1/MPEG2 Video](https://datatracker.ietf.org/doc/html/rfc2250)|MPEG-1 video, MPEG-2 audio, MPEG-TS payload formats|
|[RFC2435, RTP Payload Format for JPEG-compressed Video](https://datatracker.ietf.org/doc/html/rfc2435)|M-JPEG payload format|
|[RFC7587, RTP Payload Format for the Opus Speech and Audio Codec](https://datatracker.ietf.org/doc/html/rfc7587)|Opus payload format|
|[RFC5215, RTP Payload Format for Vorbis Encoded Audio](https://datatracker.ietf.org/doc/html/rfc5215)|Vorbis payload format|
|[RFC4184, RTP Payload Format for AC-3 Audio](https://datatracker.ietf.org/doc/html/rfc4184)|AC-3 payload format|
|[RFC6416, RTP Payload Format for MPEG-4 Audio/Visual Streams](https://datatracker.ietf.org/doc/html/rfc6416)|MPEG-4 audio payload format|
|[RFC5574, RTP Payload Format for the Speex Codec](https://datatracker.ietf.org/doc/html/rfc5574)|Speex payload format|
|[RFC3551, RTP Profile for Audio and Video Conferences with Minimal Control](https://datatracker.ietf.org/doc/html/rfc3551)|G726, G722, G711 payload formats|
|[RFC3190, RTP Payload Format for 12-bit DAT Audio and 20- and 24-bit Linear Sampled Audio](https://datatracker.ietf.org/doc/html/rfc3190)|LPCM payload format|
|[Codec specifications](https://github.com/bluenviron/mediacommon#specifications)|codecs|
|[Golang project layout](https://github.com/golang-standards/project-layout)|project layout|

## Related projects

* [MediaMTX](https://github.com/bluenviron/mediamtx)
* [gohlslib](https://github.com/bluenviron/gohlslib)
* [mediacommon](https://github.com/bluenviron/mediacommon)
* [pion/sdp (SDP library used internally)](https://github.com/pion/sdp)
* [pion/rtp (RTP library used internally)](https://github.com/pion/rtp)
* [pion/rtcp (RTCP library used internally)](https://github.com/pion/rtcp)
