package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackOpusNew(t *testing.T) {
	_, err := NewTrackOpus(96, 48000, 2)
	require.NoError(t, err)
}

func TestTrackOpusNewFromMediaDescription(t *testing.T) {
	for _, ca := range []struct {
		name  string
		md    *psdp.MediaDescription
		track *TrackOpus
	}{
		{
			"generic",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 opus/48000/2",
					},
					{
						Key:   "fmtp",
						Value: "96 sprop-stereo=1",
					},
				},
			},
			&TrackOpus{
				payloadType:  96,
				sampleRate:   48000,
				channelCount: 2,
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			track, err := newTrackOpusFromMediaDescription(96, ca.md)
			require.NoError(t, err)
			require.Equal(t, ca.track, track)
		})
	}
}

func TestTrackOpusNewFromMediaDescriptionErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		md   *psdp.MediaDescription
		err  string
	}{
		{
			"missing rtpmap",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{},
			},
			"rtpmap attribute is missing",
		},
		{
			"invalid rtpmap",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96",
					},
				},
			},
			"invalid rtpmap (96)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := newTrackOpusFromMediaDescription(96, ca.md)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestTrackOpusMediaDescription(t *testing.T) {
	track, err := NewTrackOpus(96, 48000, 2)
	require.NoError(t, err)

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"96"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "96 opus/48000/2",
			},
			{
				Key:   "fmtp",
				Value: "96 sprop-stereo=1",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.mediaDescription())
}
