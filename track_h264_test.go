package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackH264New(t *testing.T) {
	tr, err := NewTrackH264(96, &TrackConfigH264{
		SPS: []byte{
			0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
			0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
			0x00, 0x03, 0x00, 0x3d, 0x08,
		},
		PPS: []byte{
			0x68, 0xee, 0x3c, 0x80,
		},
	})
	require.NoError(t, err)
	require.Equal(t, &Track{
		Media: &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "video",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{"96"},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "rtpmap",
					Value: "96 H264/90000",
				},
				{
					Key: "fmtp",
					Value: "96 packetization-mode=1; " +
						"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C",
				},
			},
		},
	}, tr)
}

func TestTrackIsH264(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
	}{
		{
			"standard",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key: "fmtp",
							Value: "96 packetization-mode=1; " +
								"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C",
						},
					},
				},
			},
		},
		{
			"space at the end rtpmap",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000 ",
						},
					},
				},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			require.Equal(t, true, ca.track.IsH264())
		})
	}
}

func TestTrackExtractConfigH264(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
		conf  *TrackConfigH264
	}{
		{
			"generic",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key: "fmtp",
							Value: "96 packetization-mode=1; " +
								"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C",
						},
					},
				},
			},
			&TrackConfigH264{
				SPS: []byte{
					0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
					0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
					0x00, 0x03, 0x00, 0x3d, 0x08,
				},
				PPS: []byte{
					0x68, 0xee, 0x3c, 0x80,
				},
			},
		},
		{
			"vlc rtsp server",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key: "fmtp",
							Value: "96 packetization-mode=1;profile-level-id=64001f;" +
								"sprop-parameter-sets=Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,aOvjyyLA;",
						},
					},
				},
			},
			&TrackConfigH264{
				SPS: []byte{
					0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50,
					0x05, 0xbb, 0x01, 0x6c, 0x80, 0x00, 0x00, 0x03,
					0x00, 0x80, 0x00, 0x00, 0x1e, 0x07, 0x8c, 0x18,
					0xcb,
				},
				PPS: []byte{
					0x68, 0xeb, 0xe3, 0xcb, 0x22, 0xc0,
				},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			conf, err := ca.track.ExtractConfigH264()
			require.NoError(t, err)
			require.Equal(t, ca.conf, conf)
		})
	}
}

func TestTrackConfigH264Errors(t *testing.T) {
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
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
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
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96",
						},
					},
				},
			},
			"invalid fmtp attribute (96)",
		},
		{
			"fmtp without key",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 packetization-mode",
						},
					},
				},
			},
			"invalid fmtp attribute (96 packetization-mode)",
		},
		{
			"missing sprop-parameter-set",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 packetization-mode=1",
						},
					},
				},
			},
			"sprop-parameter-sets is missing (96 packetization-mode=1)",
		},
		{
			"invalid sprop-parameter-set 1",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 sprop-parameter-sets=aaaaaa",
						},
					},
				},
			},
			"invalid sprop-parameter-sets (96 sprop-parameter-sets=aaaaaa)",
		},
		{
			"invalid sprop-parameter-set 2",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 sprop-parameter-sets=aaaaaa,bbb",
						},
					},
				},
			},
			"invalid sprop-parameter-sets (96 sprop-parameter-sets=aaaaaa,bbb)",
		},
		{
			"invalid sprop-parameter-set 3",
			&Track{
				Media: &psdp.MediaDescription{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 sprop-parameter-sets=Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,bbb",
						},
					},
				},
			},
			"invalid sprop-parameter-sets (96 sprop-parameter-sets=Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,bbb)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ca.track.ExtractConfigH264()
			require.Equal(t, ca.err, err.Error())
		})
	}
}
