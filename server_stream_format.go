package gortsplib

import (
	"github.com/bluenviron/gortsplib/v3/pkg/format"
	"github.com/bluenviron/gortsplib/v3/pkg/rtcpsender"
)

type serverStreamFormat struct {
	format     format.Format
	rtcpSender *rtcpsender.RTCPSender
}
