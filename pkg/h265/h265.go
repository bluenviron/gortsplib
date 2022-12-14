// Package h265 contains utilities to work with the H265 codec.
package h265

const (
	// MaxNALUSize is the maximum size of a NALU.
	// with a 250 Mbps H265 video, the maximum NALU size is 2.2MB
	MaxNALUSize = 3 * 1024 * 1024

	// MaxNALUsPerGroup is the maximum number of NALUs per group.
	MaxNALUsPerGroup = 20
)
