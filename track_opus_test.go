package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackOpusNew(t *testing.T) {
	track, err := NewTrackOpus(96, &TrackConfigOpus{
		SampleRate:   48000,
		ChannelCount: 2,
	})
	require.NoError(t, err)
	require.Equal(t, &Track{
		Media: &psdp.MediaDescription{
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
	}, track)
}

func TestTrackIsOpus(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
	}{
		{
			"standard",
			&Track{
				Media: &psdp.MediaDescription{
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
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			require.Equal(t, true, ca.track.IsOpus())
		})
	}
}

func TestTrackExtractConfigOpus(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
		conf  *TrackConfigOpus
	}{
		{
			"generic",
			&Track{
				Media: &psdp.MediaDescription{
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
			},
			&TrackConfigOpus{
				SampleRate:   48000,
				ChannelCount: 2,
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			conf, err := ca.track.ExtractConfigOpus()
			require.NoError(t, err)
			require.Equal(t, ca.conf, conf)
		})
	}
}

func TestTrackConfigOpusErrors(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
		err   string
	}{
		{
			"missing rtpmap",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{},
				},
			},
			"rtpmap attribute is missing",
		},
		{
			"invalid rtpmap",
			&Track{
				Media: &psdp.MediaDescription{
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
			},
			"invalid rtpmap (96)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ca.track.ExtractConfigOpus()
			require.EqualError(t, err, ca.err)
		})
	}
}
