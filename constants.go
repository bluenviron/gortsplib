package gortsplib

const (
	// 1500 (UDP MTU) - 20 (IP header) - 8 (UDP header)
	udpMaxPayloadSize = 1472

	// 16 master key + 14 master salt
	srtpKeyLength = 30

	// 10 (HMAC SHA1 authentication tag)
	srtpOverhead = 10

	// 10 (HMAC SHA1 authentication tag) + 4 (sequence number)
	srtcpOverhead = 14
)
