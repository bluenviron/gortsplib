
# gortsplib

[![Test](https://github.com/aler9/gortsplib/workflows/test/badge.svg)](https://github.com/aler9/gortsplib/actions?query=workflow:test)
[![Lint](https://github.com/aler9/gortsplib/workflows/lint/badge.svg)](https://github.com/aler9/gortsplib/actions?query=workflow:lint)
[![Go Report Card](https://goreportcard.com/badge/github.com/aler9/gortsplib)](https://goreportcard.com/report/github.com/aler9/gortsplib)
[![CodeCov](https://codecov.io/gh/aler9/gortsplib/branch/main/graph/badge.svg)](https://codecov.io/gh/aler9/gortsplib/branch/main)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/aler9/gortsplib/v2)](https://pkg.go.dev/github.com/aler9/gortsplib/v2#pkg-index)

RTSP 1.0 client and server library for the Go programming language, written for [rtsp-simple-server](https://github.com/aler9/rtsp-simple-server).

Go &ge; 1.17 is required.

***WARNING: the main branch contains code of the v2 version. The legacy v1 version is hosted in the [v1 branch](https://github.com/aler9/gortsplib/tree/v1).***

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
  * Encode and decode format-specific frames into/from RTP packets. The following formats are supported:
    * Video: H264, H265, VP8, VP9
    * Audio: G711 (PCMA, PCMU), G722, LPCM, MPEG4-audio (AAC), Opus
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
* [client-read-format-mpeg4audio](examples/client-read-format-mpeg4audio/main.go)
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
* [client-publish-format-mpeg4audio](examples/client-publish-format-mpeg4audio/main.go)
* [client-publish-format-opus](examples/client-publish-format-opus/main.go)
* [client-publish-format-vp8](examples/client-publish-format-vp8/main.go)
* [client-publish-format-vp9](examples/client-publish-format-vp9/main.go)
* [server](examples/server/main.go)
* [server-tls](examples/server-tls/main.go)
* [server-h264-save-to-disk](examples/server-h264-save-to-disk/main.go)

## API Documentation

https://pkg.go.dev/github.com/aler9/gortsplib/v2#pkg-index

## Links

Related projects

* rtsp-simple-server https://github.com/aler9/rtsp-simple-server
* pion/sdp (SDP library used internally) https://github.com/pion/sdp
* pion/rtp (RTP library used internally) https://github.com/pion/rtp
* pion/rtcp (RTCP library used internally) https://github.com/pion/rtcp

Standards

* RTSP 1.0 https://www.rfc-editor.org/rfc/rfc2326
* RTSP 2.0 https://www.rfc-editor.org/rfc/rfc7826
* RTP Profile for Audio and Video Conferences with Minimal Control https://www.rfc-editor.org/rfc/rfc3551
* RTP Payload Format for MPEG1/MPEG2 Video https://www.rfc-editor.org/rfc/rfc2250
* RTP Payload Format for JPEG-compressed Video https://www.rfc-editor.org/rfc/rfc2435
* RTP Payload Format for H.264 Video https://www.rfc-editor.org/rfc/rfc6184
* RTP Payload Format for High Efficiency Video Coding (HEVC) https://www.rfc-editor.org/rfc/rfc7798.html
* RTP Payload Format for VP8 Video https://www.rfc-editor.org/rfc/rfc7741.html
* RTP Payload Format for VP9 Video https://datatracker.ietf.org/doc/html/draft-ietf-payload-vp9-16
* RTP Payload Format for 12-bit DAT Audio and 20- and 24-bit Linear Sampled Audio https://www.rfc-editor.org/rfc/rfc3190.html
* RTP Payload Format for the Opus Speech and Audio Codec https://www.rfc-editor.org/rfc/rfc7587.html
* RTP Payload Format for MPEG-4 Audio/Visual Streams https://www.rfc-editor.org/rfc/rfc6416
* RTP Payload Format for Transport of MPEG-4 Elementary Streams https://www.rfc-editor.org/rfc/rfc3640.html
* ITU-T Rec. H.264 (08/2021) https://www.itu.int/rec/dologin_pub.asp?lang=e&id=T-REC-H.264-202108-I!!PDF-E&type=items
* ITU-T Rec. H.265 (08/2021) https://www.itu.int/rec/dologin_pub.asp?lang=e&id=T-REC-H.265-202108-I!!PDF-E&type=items
* ISO 14496-3, Coding of audio-visual objects, part 3, Audio
* Golang project layout https://github.com/golang-standards/project-layout
