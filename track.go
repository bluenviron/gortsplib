package gortsplib

import (
	"strconv"

	"github.com/aler9/sdp-dirty/v3"
)

// Track is a track available in a certain URL.
type Track struct {
	// track id
	Id int

	// track codec and info in SDP format
	Media *sdp.MediaDescription
}

// Tracks is a list of tracks.
type Tracks []*Track

// ReadTracks reads tracks from an encoded SDP.
func ReadTracks(encodedSdp []byte) (Tracks, error) {
	sdpd := &sdp.SessionDescription{}
	err := sdpd.Unmarshal(encodedSdp)
	if err != nil {
		return nil, err
	}

	ts := make(Tracks, len(sdpd.MediaDescriptions))
	for i, media := range sdpd.MediaDescriptions {
		ts[i] = &Track{
			Id:    i,
			Media: media,
		}
	}

	return ts, nil
}

// Write writes tracks in SDP format.
func (ts Tracks) Write() []byte {
	sout := &sdp.SessionDescription{
		SessionName: func() *sdp.SessionName {
			ret := sdp.SessionName("Stream")
			return &ret
		}(),
		Origin: &sdp.Origin{
			Username:       "-",
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "127.0.0.1",
		},
		TimeDescriptions: []sdp.TimeDescription{
			{Timing: sdp.Timing{0, 0}},
		},
	}

	for i, track := range ts {
		mout := &sdp.MediaDescription{
			MediaName: sdp.MediaName{
				Media:   track.Media.MediaName.Media,
				Protos:  []string{"RTP", "AVP"}, // override protocol
				Formats: track.Media.MediaName.Formats,
			},
			Bandwidth: track.Media.Bandwidth,
			Attributes: func() []sdp.Attribute {
				var ret []sdp.Attribute

				for _, attr := range track.Media.Attributes {
					if attr.Key == "rtpmap" || attr.Key == "fmtp" {
						ret = append(ret, attr)
					}
				}

				// control attribute is the path that is appended
				// to the stream path in SETUP
				ret = append(ret, sdp.Attribute{
					Key:   "control",
					Value: "trackID=" + strconv.FormatInt(int64(i), 10),
				})

				return ret
			}(),
		}
		sout.MediaDescriptions = append(sout.MediaDescriptions, mout)
	}

	byts, _ := sout.Marshal()
	return byts
}
