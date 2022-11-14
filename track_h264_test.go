package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackH264Attributes(t *testing.T) {
	track := &TrackH264{
		PayloadType:       96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
	require.Equal(t, []byte{0x01, 0x02}, track.SafeSPS())
	require.Equal(t, []byte{0x03, 0x04}, track.SafePPS())

	track.SafeSetSPS([]byte{0x07, 0x08})
	track.SafeSetPPS([]byte{0x09, 0x0A})
	require.Equal(t, []byte{0x07, 0x08}, track.SafeSPS())
	require.Equal(t, []byte{0x09, 0x0A}, track.SafePPS())
}

func TestTrackH264GetSPSPPSErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		md   *psdp.MediaDescription
		err  string
	}{
		{
			"missing fmtp",
			&psdp.MediaDescription{
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
			"fmtp attribute is missing",
		},
		{
			"invalid fmtp",
			&psdp.MediaDescription{
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
			"invalid fmtp attribute (96)",
		},
		{
			"fmtp without key",
			&psdp.MediaDescription{
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
			"invalid fmtp attribute (96 packetization-mode)",
		},
		{
			"missing sprop-parameter-set",
			&psdp.MediaDescription{
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
			"sprop-parameter-sets is missing (96 packetization-mode=1)",
		},
		{
			"invalid sprop-parameter-set 1",
			&psdp.MediaDescription{
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
			"invalid sprop-parameter-sets (96 sprop-parameter-sets=aaaaaa)",
		},
		{
			"invalid sprop-parameter-set 2",
			&psdp.MediaDescription{
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
			"invalid sprop-parameter-sets (96 sprop-parameter-sets=aaaaaa,bbb)",
		},
		{
			"invalid sprop-parameter-set 3",
			&psdp.MediaDescription{
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
			"invalid sprop-parameter-sets (96 sprop-parameter-sets=Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,bbb)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var tr TrackH264
			err := tr.fillParamsFromMediaDescription(ca.md)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestTrackH264Clone(t *testing.T) {
	track := &TrackH264{
		PayloadType:       96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackH264MediaDescription(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		track := &TrackH264{
			PayloadType: 96,
			SPS: []byte{
				0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
				0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
				0x00, 0x03, 0x00, 0x3d, 0x08,
			},
			PPS: []byte{
				0x68, 0xee, 0x3c, 0x80,
			},
			PacketizationMode: 1,
		}

		require.Equal(t, &psdp.MediaDescription{
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
				{
					Key:   "control",
					Value: "",
				},
			},
		}, track.MediaDescription())
	})

	t.Run("no sps/pps", func(t *testing.T) {
		track := &TrackH264{
			PayloadType:       96,
			PacketizationMode: 1,
		}

		require.Equal(t, &psdp.MediaDescription{
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
				{
					Key:   "control",
					Value: "",
				},
			},
		}, track.MediaDescription())
	})
}
