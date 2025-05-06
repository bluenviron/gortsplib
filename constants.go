package gortsplib

const (
	// same size as GStreamer's rtspsrc
	udpKernelReadBufferSize = 0x80000

	// 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header)
	udpMaxPayloadSize = 1472
	
	// HTTP tunnel specific constants
	httpTunnelCookieName        = "x-sessioncookie"
	httpTunnelContentType       = "application/x-rtsp-tunnelled"
	httpTunnelGetSuffix         = "?protocol=get"
	httpTunnelPostSuffix        = "?protocol=post"
	httpTunnelDefaultBufferSize = 4096
)
