package gortsplib

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/notedit/rtmp/codec/aac"
	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/sdp"
)

// Track is a track available in a certain URL.
type Track struct {
	// track id
	Id int

	// track codec and info in SDP format
	Media *psdp.MediaDescription
}

// NewTrackH264 initializes an H264 track.
func NewTrackH264(id int, sps []byte, pps []byte) (*Track, error) {
	spropParameterSets := base64.StdEncoding.EncodeToString(sps) +
		"," + base64.StdEncoding.EncodeToString(pps)
	profileLevelId := strings.ToUpper(hex.EncodeToString(sps[1:4]))

	typ := strconv.FormatInt(int64(96+id), 10)

	return &Track{
		Id: id,
		Media: &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "video",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{typ},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "rtpmap",
					Value: typ + " H264/90000",
				},
				{
					Key: "fmtp",
					Value: typ + " packetization-mode=1; " +
						"sprop-parameter-sets=" + spropParameterSets + "; " +
						"profile-level-id=" + profileLevelId,
				},
			},
		},
	}, nil
}

// NewTrackAac initializes an AAC track.
func NewTrackAac(id int, config []byte) (*Track, error) {
	codec, err := aac.FromMPEG4AudioConfigBytes(config)
	if err != nil {
		return nil, err
	}

	// https://github.com/notedit/rtmp/blob/6e314ac5b29611431f8fb5468596b05815743c10/codec/aac/aac.go#L106
	channelCount, err := func() (int, error) {
		if codec.Config.ChannelConfig >= 1 && codec.Config.ChannelConfig <= 6 {
			return int(codec.Config.ChannelConfig), nil
		}

		if codec.Config.ChannelConfig == 8 {
			return 7, nil
		}

		return 0, fmt.Errorf("unsupported channel config: %v", codec.Config.ChannelConfig)
	}()
	if err != nil {
		return nil, err
	}

	typ := strconv.FormatInt(int64(96+id), 10)

	return &Track{
		Id: id,
		Media: &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "audio",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{typ},
			},
			Attributes: []psdp.Attribute{
				{
					Key: "rtpmap",
					Value: typ + " MPEG4-GENERIC/" + strconv.FormatInt(int64(codec.Config.SampleRate), 10) +
						"/" + strconv.FormatInt(int64(channelCount), 10),
				},
				{
					Key: "fmtp",
					Value: typ + " profile-level-id=1; " +
						"mode=AAC-hbr; " +
						"sizelength=13; " +
						"indexlength=3; " +
						"indexdeltalength=3; " +
						"config=" + hex.EncodeToString(config),
				},
			},
		},
	}, nil
}

// Tracks is a list of tracks.
type Tracks []*Track

// ReadTracks reads tracks from an encoded SDP.
func ReadTracks(byts []byte) (Tracks, error) {
	desc := sdp.SessionDescription{}
	err := desc.Unmarshal(byts)
	if err != nil {
		return nil, err
	}

	ts := make(Tracks, len(desc.MediaDescriptions))
	for i, media := range desc.MediaDescriptions {
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
			Address:     &psdp.Address{Address: "0.0.0.0"},
		},
		TimeDescriptions: []psdp.TimeDescription{
			{Timing: psdp.Timing{0, 0}},
		},
	}

	for i, track := range ts {
		mout := &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   track.Media.MediaName.Media,
				Protos:  []string{"RTP", "AVP"}, // override protocol
				Formats: track.Media.MediaName.Formats,
			},
			Bandwidth: track.Media.Bandwidth,
			Attributes: func() []psdp.Attribute {
				var ret []psdp.Attribute

				for _, attr := range track.Media.Attributes {
					if attr.Key == "rtpmap" || attr.Key == "fmtp" {
						ret = append(ret, attr)
					}
				}

				// control attribute is the path that is appended
				// to the stream path in SETUP
				ret = append(ret, psdp.Attribute{
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
