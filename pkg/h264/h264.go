// Package h264 contains utilities to work with the H264 codec.
package h264

const (
	// with a 250 Mbps H264 video, the maximum NALU size is 2.2MB
	maxNALUSize = 3 * 1024 * 1024
)
