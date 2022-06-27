package gortsplib

import (
	"fmt"
	"strconv"
	"strings"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/sdp"
)

// Tracks is a list of tracks.
type Tracks []Track

// Unmarshal decodes tracks from the SDP format. It returns the decoded SDP.
func (ts *Tracks) Unmarshal(byts []byte, skipGenericTracksWithoutClockRate bool) (*sdp.SessionDescription, error) {
	var sd sdp.SessionDescription
	err := sd.Unmarshal(byts)
	if err != nil {
		return nil, err
	}

	*ts = nil

	for i, md := range sd.MediaDescriptions {
		t, err := newTrackFromMediaDescription(md)
		if err != nil {
			if skipGenericTracksWithoutClockRate &&
				strings.HasPrefix(err.Error(), "unable to get clock rate") {
				continue
			}
			return nil, fmt.Errorf("unable to parse track %d: %s", i+1, err)
		}

		*ts = append(*ts, t)
	}

	if *ts == nil {
		return nil, fmt.Errorf("no valid tracks found")
	}

	return &sd, nil
}

func (ts Tracks) clone() Tracks {
	ret := make(Tracks, len(ts))
	for i, track := range ts {
		ret[i] = track.clone()
	}
	return ret
}

func (ts Tracks) setControls() {
	for i, t := range ts {
		t.SetControl("trackID=" + strconv.FormatInt(int64(i), 10))
	}
}

// Marshal encodes tracks in the SDP format.
func (ts Tracks) Marshal(multicast bool) []byte {
	address := "0.0.0.0"
	if multicast {
		address = "224.1.0.0"
	}

	sout := &sdp.SessionDescription{
		SessionName: psdp.SessionName("Stream"),
		Origin: psdp.Origin{
			Username:       "-",
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "127.0.0.1",
		},
		// required by Darwin Streaming Server
		ConnectionInformation: &psdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &psdp.Address{Address: address},
		},
		TimeDescriptions: []psdp.TimeDescription{
			{Timing: psdp.Timing{0, 0}}, //nolint:govet
		},
	}

	for _, track := range ts {
		sout.MediaDescriptions = append(sout.MediaDescriptions, track.MediaDescription())
	}

	byts, _ := sout.Marshal()
	return byts
}
