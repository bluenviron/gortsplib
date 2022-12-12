package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/mpeg4audio"
	"github.com/aler9/gortsplib/pkg/url"
)

func TestTrackNewFromMediaDescription(t *testing.T) {
	for _, ca := range []struct {
		name  string
		md    *psdp.MediaDescription
		track Track
	}{
		{
			"pcma",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"8"},
				},
			},
			&TrackG711{},
		},
		{
			"pcmu",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"0"},
				},
			},
			&TrackG711{
				MULaw: true,
			},
		},
		{
			"g722",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"9"},
				},
			},
			&TrackG722{},
		},
		{
			"lpcm 8",
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
			&TrackLPCM{
				PayloadType:  97,
				BitDepth:     8,
				SampleRate:   48000,
				ChannelCount: 2,
			},
		},
		{
			"lpcm 16",
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
			&TrackLPCM{
				PayloadType:  97,
				BitDepth:     16,
				SampleRate:   96000,
				ChannelCount: 2,
			},
		},
		{
			"lpcm 16 with no explicit channel",
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
			&TrackLPCM{
				PayloadType:  97,
				BitDepth:     16,
				SampleRate:   16000,
				ChannelCount: 1,
			},
		},
		{
			"lpcm 24",
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
			&TrackLPCM{
				PayloadType:  98,
				BitDepth:     24,
				SampleRate:   44100,
				ChannelCount: 4,
			},
		},
		{
			"mpeg audio",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"14"},
				},
			},
			&TrackMPEG2Audio{},
		},
		{
			"aac",
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
			&TrackMPEG4Audio{
				PayloadType: 96,
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
			"aac vlc rtsp server",
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
			&TrackMPEG4Audio{
				PayloadType: 96,
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
			"aac without indexlength",
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
			&TrackMPEG4Audio{
				PayloadType: 96,
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
			"vorbis",
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
			&TrackVorbis{
				PayloadType:   96,
				SampleRate:    44100,
				ChannelCount:  2,
				Configuration: []byte{0x01, 0x02, 0x03, 0x04},
			},
		},
		{
			"opus",
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
				PayloadType:  96,
				SampleRate:   48000,
				ChannelCount: 2,
			},
		},
		{
			"jpeg",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"26"},
				},
			},
			&TrackJPEG{},
		},
		{
			"mpeg video",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"32"},
				},
			},
			&TrackMPEG2Video{},
		},
		{
			"h264",
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
			&TrackH264{
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
			},
		},
		{
			"h264 with a space at the end of rtpmap",
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
			&TrackH264{
				PayloadType: 96,
			},
		},
		{
			"h264 vlc rtsp server",
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
			&TrackH264{
				PayloadType: 96,
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
			"h264 sprop-parameter-sets with extra data",
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
			&TrackH264{
				PayloadType: 96,
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
			"h265",
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
			&TrackH265{
				PayloadType: 96,
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
			"vp8",
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
			&TrackVP8{
				PayloadType: 96,
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
			"vp9",
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
			&TrackVP9{
				PayloadType: 96,
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
			"multiple formats",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96", "98"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 H264/90000",
					},
					{
						Key:   "rtpmap",
						Value: "98 MetaData",
					},
					{
						Key: "rtcp-mux",
					},
					{
						Key: "fmtp",
						Value: "96 packetization-mode=1;profile-level-id=4d002a;" +
							"sprop-parameter-sets=Z00AKp2oHgCJ+WbgICAgQA==,aO48gA==",
					},
				},
			},
			&TrackGeneric{
				Media: "video",
				Payloads: []TrackGenericPayload{
					{
						Type:   96,
						RTPMap: "H264/90000",
						FMTP:   "packetization-mode=1;profile-level-id=4d002a;sprop-parameter-sets=Z00AKp2oHgCJ+WbgICAgQA==,aO48gA==",
					},
					{
						Type:   98,
						RTPMap: "MetaData",
					},
				},
				clockRate: 90000,
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			track, err := newTrackFromMediaDescription(ca.md)
			require.NoError(t, err)
			require.Equal(t, ca.track, track)
		})
	}
}

func TestTrackNewFromMediaDescriptionErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		md   *psdp.MediaDescription
		err  string
	}{
		{
			"no formats",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{},
				},
			},
			"no media formats found",
		},
		{
			"aac missing fmtp",
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
			"aac invalid fmtp",
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
			"aac fmtp without key",
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
			"aac missing config",
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
			"aac invalid config 1",
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
			"aac invalid config 2",
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
			"aac missing sizelength",
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
			"sizelength is missing (96 profile-level-id=1; config=1190)",
		},
		{
			"opus invalid 1",
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
			"opus invalid 2",
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
			"opus invalid 3",
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
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := newTrackFromMediaDescription(ca.md)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestTrackURL(t *testing.T) {
	for _, ca := range []struct {
		name    string
		sdp     []byte
		baseURL *url.URL
		ur      *url.URL
	}{
		{
			"missing control",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
		},
		{
			"absolute control",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:rtsp://localhost/path/trackID=7"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/trackID=7"),
		},
		{
			"relative control",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/trackID=5"),
		},
		{
			"relative control, subpath",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/trackID=5"),
		},
		{
			"relative control, url without slash",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/trackID=5"),
		},
		{
			"relative control, url with query",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"test?user=tmp&password=BagRep1&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, url with special chars and query",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, url with query without question mark",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, control is query",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:?ctype=video"),
			mustParseURL("rtsp://192.168.1.99:554/test"),
			mustParseURL("rtsp://192.168.1.99:554/test?ctype=video"),
		},
		{
			"relative control, control is query and no path",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:?ctype=video"),
			mustParseURL("rtsp://192.168.1.99:554/"),
			mustParseURL("rtsp://192.168.1.99:554/?ctype=video"),
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var tracks Tracks
			_, err := tracks.Unmarshal(ca.sdp)
			require.NoError(t, err)
			ur, err := tracks[0].url(ca.baseURL)
			require.NoError(t, err)
			require.Equal(t, ca.ur, ur)
		})
	}
}

func TestTrackURLError(t *testing.T) {
	track := &TrackH264{}
	_, err := track.url(nil)
	require.EqualError(t, err, "Content-Base header not provided")
}
