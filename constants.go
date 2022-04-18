package gortsplib

const (
	tcpReadBufferSize = 4096

	// this must fit an entire H264 NALU and a RTP header.
	// with a 250 Mbps H264 video, the maximum NALU size is 2.2MB
	tcpMaxFramePayloadSize = 3 * 1024 * 1024

	// same size as GStreamer's rtspsrc
	udpKernelReadBufferSize = 0x80000

	// 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header)
	maxPacketSize = 1472

	// same size as GStreamer's rtspsrc
	multicastTTL = 16
)
