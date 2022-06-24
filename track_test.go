package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/aac"
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
			&TrackPCMA{},
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
			&TrackPCMU{},
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
			&TrackMpegAudio{},
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
			&TrackAAC{
				PayloadType: 96,
				Config: &aac.MPEG4AudioConfig{
					Type:         2,
					SampleRate:   48000,
					ChannelCount: 2,
				},
				SizeLength:       13,
				IndexLength:      3,
				IndexDeltaLength: 3,
			},
		},
		{
			"aac uppercase",
			&psdp.MediaDescription{
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
			&TrackAAC{
				PayloadType: 96,
				Config: &aac.MPEG4AudioConfig{
					Type:         2,
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
			&TrackAAC{
				PayloadType: 96,
				Config: &aac.MPEG4AudioConfig{
					Type:         2,
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
			&TrackAAC{
				PayloadType: 96,
				Config: &aac.MPEG4AudioConfig{
					Type:         2,
					SampleRate:   48000,
					ChannelCount: 2,
				},
				SizeLength:       13,
				IndexLength:      0,
				IndexDeltaLength: 0,
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
			&TrackMpegVideo{},
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
							"sprop-sps=QgEBAWAAAAMAkAAAAwAAAwB4oAPAgBDllmZpJMrgEAAAAwAQAAADAeCA; sprop-pps=RAHBcrRiQA==",
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
					Formats: []string{"98", "96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "98 H265/90000",
					},
					{
						Key: "fmtp",
						Value: "98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
							"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
					},
				},
			},
			&TrackGeneric{
				Media:   "video",
				Formats: []string{"98", "96"},
				RTPMap:  "98 H265/90000",
				FMTP: "98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
					"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
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
			"unable to get clock rate: no formats provided",
		},
		{
			"no rtpmap",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"90"},
				},
			},
			"unable to get clock rate: attribute 'rtpmap' not found",
		},
		{
			"invalid clockrate 1",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
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
			"unable to get clock rate: invalid rtpmap (96)",
		},
		{
			"invalid clockrate 2",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 mpeg4-generic",
					},
				},
			},
			"unable to get clock rate: invalid rtpmap (96 mpeg4-generic)",
		},
		{
			"invalid clockrate 3",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "96 mpeg4-generic/aa",
					},
				},
			},
			"unable to get clock rate: strconv.ParseInt: parsing \"aa\": invalid syntax",
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
			"invalid rtpmap (opus/48000)",
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
			tracks, _, err := ReadTracks(ca.sdp, false)
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
