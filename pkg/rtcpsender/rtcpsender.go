// Package rtcpsender contains a utility to generate RTCP sender reports.
package rtcpsender

import "github.com/bluenviron/gortsplib/v5/pkg/rtpsender"

// RTCPSender is a utility to send RTP packets.
//
// Deprecated: replaced by rtpsender.Sender
type RTCPSender = rtpsender.Sender
