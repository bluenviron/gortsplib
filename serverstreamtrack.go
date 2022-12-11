package gortsplib

import (
	"github.com/aler9/gortsplib/pkg/rtcpsender"
	"github.com/aler9/gortsplib/pkg/track"
)

type serverStreamTrack struct {
	track      track.Track
	rtcpSender *rtcpsender.RTCPSender
}
