package gortsplib

import (
	"github.com/aler9/gortsplib/v2/pkg/rtcpsender"
	"github.com/aler9/gortsplib/v2/pkg/track"
)

type serverStreamTrack struct {
	track      track.Track
	rtcpSender *rtcpsender.RTCPSender
}
