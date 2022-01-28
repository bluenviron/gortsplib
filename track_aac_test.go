package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackAACNew(t *testing.T) {
	_, err := NewTrackAAC(96, 2, 48000, 2, nil)
	require.NoError(t, err)
}

func TestTrackAACNewFromMediaDescription(t *testing.T) {
	for _, ca := range []struct {
		name  string
		md    *psdp.MediaDescription
		track *TrackAAC
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
						Value: "96 mpeg4-generic/48000/2",
					},
					{
						Key:   "fmtp",
						Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
					},
				},
			},
			&TrackAAC{
				payloadType:  96,
				sampleRate:   48000,
				channelCount: 2,
				mpegConf:     []byte{0x11, 0x90},
			},
		},
		{
			"vlc rtsp server",
			&psdp.MediaDescription{
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
			&TrackAAC{
				payloadType:  96,
				sampleRate:   48000,
				channelCount: 2,
				mpegConf:     []byte{0x11, 0x90},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			track, err := newTrackAACFromMediaDescription(96, ca.md)
			require.NoError(t, err)
			require.Equal(t, ca.track, track)
		})
	}
}

func TestTrackAACNewFromMediaDescriptionErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		md   *psdp.MediaDescription
		err  string
	}{
		{
			"missing fmtp",
			&psdp.MediaDescription{
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
			"fmtp attribute is missing",
		},
		{
			"invalid fmtp",
			&psdp.MediaDescription{
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
			"invalid fmtp (96)",
		},
		{
			"fmtp without key",
			&psdp.MediaDescription{
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
			"invalid fmtp (96 profile-level-id)",
		},
		{
			"missing config",
			&psdp.MediaDescription{
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
			"config is missing (96 profile-level-id=1)",
		},
		{
			"invalid config",
			&psdp.MediaDescription{
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
			"invalid AAC config (zz)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := newTrackAACFromMediaDescription(96, ca.md)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestTrackAACMediaDescription(t *testing.T) {
	track, err := NewTrackAAC(96, 2, 48000, 2, nil)
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
				Value: "96 mpeg4-generic/48000/2",
			},
			{
				Key:   "fmtp",
				Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.mediaDescription())
}
