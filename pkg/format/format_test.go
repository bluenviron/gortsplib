package format

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/v2/pkg/mpeg4audio"
)

func TestNewFromMediaDescription(t *testing.T) {
	for _, ca := range []struct {
		name   string
		md     *psdp.MediaDescription
		format Format
	}{
		{
			"audio g711 pcma",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"8"},
				},
			},
			&G711{},
		},
		{
			"audio g711 pcmu",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"0"},
				},
			},
			&G711{
				MULaw: true,
			},
		},
		{
			"audio g722",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"9"},
				},
			},
			&G722{},
		},
		{
			"audio lpcm 8",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"97"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "97 L8/48000/2",
					},
				},
			},
			&LPCM{
				PayloadTyp:   97,
				BitDepth:     8,
				SampleRate:   48000,
				ChannelCount: 2,
			},
		},
		{
			"audio lpcm 16",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"97"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "97 L16/96000/2",
					},
				},
			},
			&LPCM{
				PayloadTyp:   97,
				BitDepth:     16,
				SampleRate:   96000,
				ChannelCount: 2,
			},
		},
		{
			"audio lpcm 16 with no explicit channel",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"97"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "97 L16/16000",
					},
				},
			},
			&LPCM{
				PayloadTyp:   97,
				BitDepth:     16,
				SampleRate:   16000,
				ChannelCount: 1,
			},
		},
		{
			"audio lpcm 24",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"98"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "98 L24/44100/4",
					},
				},
			},
			&LPCM{
				PayloadTyp:   98,
				BitDepth:     24,
				SampleRate:   44100,
				ChannelCount: 4,
			},
		},
		{
			"audio mpeg2 audio",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"14"},
				},
			},
			&MPEG2Audio{},
		},
		{
			"audio aac",
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
						Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=11900810",
					},
					{
						Key:   "control",
						Value: "",
					},
				},
			},
			&MPEG4Audio{
				PayloadTyp: 96,
				Config: &mpeg4audio.Config{
					Type:         mpeg4audio.ObjectTypeAACLC,
					SampleRate:   48000,
					ChannelCount: 2,
				},
				SizeLength:       13,
				IndexLength:      3,
				IndexDeltaLength: 3,
			},
		},
		{
			"audio aac vlc rtsp server",
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
			&MPEG4Audio{
				PayloadTyp: 96,
				Config: &mpeg4audio.Config{
					Type:         mpeg4audio.ObjectTypeAACLC,
					SampleRate:   48000,
					ChannelCount: 2,
				},
				SizeLength:       13,
				IndexLength:      3,
				IndexDeltaLength: 3,
			},
		},
		{
			"audio aac without indexlength",
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
						Value: "96 streamtype=3;profile-level-id=14;mode=AAC-hbr;config=1190;sizeLength=13",
					},
					{
						Key:   "control",
						Value: "",
					},
				},
			},
			&MPEG4Audio{
				PayloadTyp: 96,
				Config: &mpeg4audio.Config{
					Type:         mpeg4audio.ObjectTypeAACLC,
					SampleRate:   48000,
					ChannelCount: 2,
				},
				SizeLength:       13,
				IndexLength:      0,
				IndexDeltaLength: 0,
			},
		},
		{
			"audio vorbis",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 VORBIS/44100/2",
					},
					{
						Key:   "fmtp",
						Value: "96 configuration=AQIDBA==",
					},
				},
			},
			&Vorbis{
				PayloadTyp:    96,
				SampleRate:    44100,
				ChannelCount:  2,
				Configuration: []byte{0x01, 0x02, 0x03, 0x04},
			},
		},
		{
			"audio opus",
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
			&Opus{
				PayloadTyp:   96,
				SampleRate:   48000,
				ChannelCount: 2,
			},
		},
		{
			"video jpeg",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"26"},
				},
			},
			&MJPEG{},
		},
		{
			"video mpeg2 video",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"32"},
				},
			},
			&MPEG2Video{},
		},
		{
			"video h264",
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
						Key: "fmtp",
						Value: "96 packetization-mode=1; " +
							"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C",
					},
				},
			},
			&H264{
				PayloadTyp: 96,
				SPS: []byte{
					0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
					0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
					0x00, 0x03, 0x00, 0x3d, 0x08,
				},
				PPS: []byte{
					0x68, 0xee, 0x3c, 0x80,
				},
				PacketizationMode: 1,
			},
		},
		{
			"video h264 with a space at the end of rtpmap",
			&psdp.MediaDescription{
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
			&H264{
				PayloadTyp: 96,
			},
		},
		{
			"video h264 vlc rtsp server",
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
						Key: "fmtp",
						Value: "96 packetization-mode=1;profile-level-id=64001f;" +
							"sprop-parameter-sets=Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,aOvjyyLA;",
					},
				},
			},
			&H264{
				PayloadTyp: 96,
				SPS: []byte{
					0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50,
					0x05, 0xbb, 0x01, 0x6c, 0x80, 0x00, 0x00, 0x03,
					0x00, 0x80, 0x00, 0x00, 0x1e, 0x07, 0x8c, 0x18,
					0xcb,
				},
				PPS: []byte{
					0x68, 0xeb, 0xe3, 0xcb, 0x22, 0xc0,
				},
				PacketizationMode: 1,
			},
		},
		{
			"video h264 sprop-parameter-sets with extra data",
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
						Key: "fmtp",
						Value: "96 packetization-mode=1; " +
							"sprop-parameter-sets=Z2QAKawTMUB4BEfeA+oCAgPgAAADACAAAAZSgA==,aPqPLA==,aF6jzAMA; profile-level-id=640029",
					},
				},
			},
			&H264{
				PayloadTyp: 96,
				SPS: []byte{
					0x67, 0x64, 0x00, 0x29, 0xac, 0x13, 0x31, 0x40,
					0x78, 0x04, 0x47, 0xde, 0x03, 0xea, 0x02, 0x02,
					0x03, 0xe0, 0x00, 0x00, 0x03, 0x00, 0x20, 0x00,
					0x00, 0x06, 0x52, 0x80,
				},
				PPS: []byte{
					0x68, 0xfa, 0x8f, 0x2c,
				},
				PacketizationMode: 1,
			},
		},
		{
			"h264 empty sprop-parameter-sets",
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
						Value: "96 packetization-mode=1; sprop-parameter-sets=",
					},
				},
			},
			&H264{
				PayloadTyp:        96,
				PacketizationMode: 1,
			},
		},
		{
			"video h265",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 H265/90000",
					},
					{
						Key: "fmtp",
						Value: "96 sprop-vps=QAEMAf//AWAAAAMAkAAAAwAAAwB4mZgJ; " +
							"sprop-sps=QgEBAWAAAAMAkAAAAwAAAwB4oAPAgBDllmZpJMrgEAAAAwAQAAADAeCA; " +
							"sprop-pps=RAHBcrRiQA==; sprop-max-don-diff=2",
					},
				},
			},
			&H265{
				PayloadTyp: 96,
				VPS: []byte{
					0x40, 0x1, 0xc, 0x1, 0xff, 0xff, 0x1, 0x60,
					0x0, 0x0, 0x3, 0x0, 0x90, 0x0, 0x0, 0x3,
					0x0, 0x0, 0x3, 0x0, 0x78, 0x99, 0x98, 0x9,
				},
				SPS: []byte{
					0x42, 0x1, 0x1, 0x1, 0x60, 0x0, 0x0, 0x3,
					0x0, 0x90, 0x0, 0x0, 0x3, 0x0, 0x0, 0x3,
					0x0, 0x78, 0xa0, 0x3, 0xc0, 0x80, 0x10, 0xe5,
					0x96, 0x66, 0x69, 0x24, 0xca, 0xe0, 0x10, 0x0,
					0x0, 0x3, 0x0, 0x10, 0x0, 0x0, 0x3, 0x1,
					0xe0, 0x80,
				},
				PPS: []byte{
					0x44, 0x1, 0xc1, 0x72, 0xb4, 0x62, 0x40,
				},
				MaxDONDiff: 2,
			},
		},
		{
			"video vp8",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 VP8/90000",
					},
					{
						Key:   "fmtp",
						Value: "96 max-fr=123;max-fs=456",
					},
				},
			},
			&VP8{
				PayloadTyp: 96,
				MaxFR: func() *int {
					v := 123
					return &v
				}(),
				MaxFS: func() *int {
					v := 456
					return &v
				}(),
			},
		},
		{
			"video vp9",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 VP9/90000",
					},
					{
						Key:   "fmtp",
						Value: "96 max-fr=123;max-fs=456;profile-id=789",
					},
				},
			},
			&VP9{
				PayloadTyp: 96,
				MaxFR: func() *int {
					v := 123
					return &v
				}(),
				MaxFS: func() *int {
					v := 456
					return &v
				}(),
				ProfileID: func() *int {
					v := 789
					return &v
				}(),
			},
		},
		{
			"application",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "application",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"98"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "98 MetaData/80000",
					},
					{
						Key: "rtcp-mux",
					},
				},
			},
			&Generic{
				PayloadTyp: 98,
				RTPMap:     "MetaData/80000",
				ClockRat:   80000,
			},
		},
		{
			"application without clock rate",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "application",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"107"},
				},
			},
			&Generic{
				PayloadTyp: 107,
			},
		},
		{
			"generic invalid rtpmap",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "application",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"98"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "98 custom",
					},
				},
			},
			&Generic{
				PayloadTyp: 98,
				RTPMap:     "custom",
			},
		},
		{
			"generic invalid rtpmap 2",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "application",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"98"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "98 custom/aaa",
					},
				},
			},
			&Generic{
				PayloadTyp: 98,
				RTPMap:     "custom/aaa",
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			format, err := Unmarshal(ca.md, ca.md.MediaName.Formats[0])
			require.NoError(t, err)
			require.Equal(t, ca.format, format)
		})
	}
}

func TestNewFromMediaDescriptionErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		md   *psdp.MediaDescription
		err  string
	}{
		{
			"audio lpcm invalid clock",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"97"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "97 L8/",
					},
				},
			},
			"strconv.ParseInt: parsing \"\": invalid syntax",
		},
		{
			"audio lpcm invalid channels",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"97"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "97 L8/48000/",
					},
				},
			},
			"strconv.ParseInt: parsing \"\": invalid syntax",
		},
		{
			"audio aac fmtp without key",
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
			"invalid fmtp (profile-level-id)",
		},
		{
			"audio aac missing config",
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
			"config is missing (profile-level-id=1)",
		},
		{
			"audio aac invalid config 1",
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
		{
			"audio aac invalid config 2",
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
						Value: "96 profile-level-id=1; config=aa",
					},
				},
			},
			"invalid AAC config (aa)",
		},
		{
			"audio aac missing sizelength",
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
						Value: "96 profile-level-id=1; config=1190",
					},
				},
			},
			"sizelength is missing (profile-level-id=1; config=1190)",
		},
		{
			"audio aac invalid sizelength",
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
						Value: "96 profile-level-id=1; sizelength=aaa",
					},
				},
			},
			"invalid AAC SizeLength (aaa)",
		},
		{
			"audio aac invalid indexlength",
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
						Value: "96 profile-level-id=1; indexlength=aaa",
					},
				},
			},
			"invalid AAC IndexLength (aaa)",
		},
		{
			"audio aac invalid indexdeltalength",
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
						Value: "96 profile-level-id=1; indexdeltalength=aaa",
					},
				},
			},
			"invalid AAC IndexDeltaLength (aaa)",
		},
		{
			"audio vorbis missing configuration",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 VORBIS/44100/2",
					},
					{
						Key:   "fmtp",
						Value: "96 aa=bb",
					},
				},
			},
			"config is missing (aa=bb)",
		},
		{
			"audio opus invalid 1",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 opus/48000",
					},
				},
			},
			"invalid clock (48000)",
		},
		{
			"audio opus invalid 2",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 opus/aa/2",
					},
				},
			},
			"strconv.ParseInt: parsing \"aa\": invalid syntax",
		},
		{
			"audio opus invalid 3",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 opus/48000/aa",
					},
				},
			},
			"strconv.ParseInt: parsing \"aa\": invalid syntax",
		},
		{
			"video h264 invalid fmtp",
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
						Value: "96 aaa",
					},
				},
			},
			"invalid fmtp attribute (aaa)",
		},
		{
			"video h264 invalid sps",
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
						Value: "96 sprop-parameter-sets=kkk,vvv",
					},
				},
			},
			"invalid sprop-parameter-sets (kkk,vvv)",
		},
		{
			"video h264 invalid pps",
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
						Value: "96 sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,vvv",
					},
				},
			},
			"invalid sprop-parameter-sets (Z2QADKw7ULBLQgAAAwACAAADAD0I,vvv)",
		},
		{
			"video h264 invalid packetization-mode",
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
						Value: "96 packetization-mode=aaa",
					},
				},
			},
			"invalid packetization-mode (aaa)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := Unmarshal(ca.md, ca.md.MediaName.Formats[0])
			require.EqualError(t, err, ca.err)
		})
	}
}
