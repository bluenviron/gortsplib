package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackAACNew(t *testing.T) {
	track, err := NewTrackAAC(96, &TrackConfigAAC{
		Type:         2,
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
					Value: "96 mpeg4-generic/48000/2",
				},
				{
					Key:   "fmtp",
					Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
				},
			},
		},
	}, track)
}

func TestTrackIsAAC(t *testing.T) {
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
							Value: "96 mpeg4-generic/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
						},
					},
				},
			},
		},
		{
			"uppercase",
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
							Value: "96 MPEG4-GENERIC/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
						},
					},
				},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			require.Equal(t, true, ca.track.IsAAC())
		})
	}
}

func TestTrackExtractConfigAAC(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
		conf  *TrackConfigAAC
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
							Value: "96 mpeg4-generic/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
						},
					},
				},
			},
			&TrackConfigAAC{
				Type:         2,
				SampleRate:   48000,
				ChannelCount: 2,
			},
		},
		{
			"vlc rtsp server",
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
							Value: "96 mpeg4-generic/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190;",
						},
					},
				},
			},
			&TrackConfigAAC{
				Type:         2,
				SampleRate:   48000,
				ChannelCount: 2,
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			conf, err := ca.track.ExtractConfigAAC()
			require.NoError(t, err)
			require.Equal(t, ca.conf, conf)
		})
	}
}

func TestTrackConfigAACErrors(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
		err   string
	}{
		{
			"missing fmtp",
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
							Value: "96 mpeg4-generic/48000/2",
						},
					},
				},
			},
			"fmtp attribute is missing",
		},
		{
			"invalid fmtp",
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
							Value: "96 mpeg4-generic/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96",
						},
					},
				},
			},
			"invalid fmtp (96)",
		},
		{
			"fmtp without key",
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
							Value: "96 mpeg4-generic/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id",
						},
					},
				},
			},
			"invalid fmtp (96 profile-level-id)",
		},
		{
			"missing config",
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
							Value: "96 mpeg4-generic/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=1",
						},
					},
				},
			},
			"config is missing (96 profile-level-id=1)",
		},
		{
			"invalid config",
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
							Value: "96 mpeg4-generic/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=1; config=zz",
						},
					},
				},
			},
			"invalid AAC config (zz)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ca.track.ExtractConfigAAC()
			require.Equal(t, ca.err, err.Error())
		})
	}
}
