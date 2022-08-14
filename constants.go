package gortsplib

const (
	// same size as GStreamer's rtspsrc
	udpKernelReadBufferSize = 0x80000

	// 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header)
	maxPacketSize = 1472

	// same size as GStreamer's rtspsrc
	multicastTTL = 16
)
