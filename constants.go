package gortsplib

const (
	tcpReadBufferSize       = 4096
	tcpMaxFramePayloadSize  = 60 * 1024 * 1024
	udpKernelReadBufferSize = 0x80000 // same size as GStreamer's rtspsrc
	udpReadBufferSize       = 1472    // 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header)
	multicastTTL            = 16
)
