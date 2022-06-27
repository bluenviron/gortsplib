// Package h264 contains utilities to work with the H264 codec.
package h264

const (
	// MaxNALUSize is the maximum size of a NALU.
	// with a 250 Mbps H264 video, the maximum NALU size is 2.2MB
	MaxNALUSize = 3 * 1024 * 1024

	// MaxNALUsPerGroup is the maximum number of NALUs per group.
	MaxNALUsPerGroup = 20
)
