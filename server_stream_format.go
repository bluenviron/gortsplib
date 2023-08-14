package gortsplib

import (
	"github.com/bluenviron/gortsplib/v4/pkg/formats"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpsender"
)

type serverStreamFormat struct {
	format     formats.Format
	rtcpSender *rtcpsender.RTCPSender
}
