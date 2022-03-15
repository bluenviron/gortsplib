package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
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
			&TrackGeneric{
				clockRate: 8000,
				media:     "audio",
				formats:   []string{"8"},
			},
		},
		{
			"pcmu",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Port:    psdp.RangedPort{Value: 49170},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"0"},
				},
			},
			&TrackGeneric{
				clockRate: 8000,
				media:     "audio",
				formats:   []string{"0"},
			},
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
						Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
					},
				},
			},
			&TrackAAC{
				payloadType:  96,
				typ:          2,
				sampleRate:   48000,
				channelCount: 2,
				mpegConf:     []byte{0x11, 0x90},
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
				payloadType:  96,
				typ:          2,
				sampleRate:   48000,
				channelCount: 2,
				mpegConf:     []byte{0x11, 0x90},
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
				payloadType:  96,
				typ:          2,
				sampleRate:   48000,
				channelCount: 2,
				mpegConf:     []byte{0x11, 0x90},
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
				payloadType:  96,
				sampleRate:   48000,
				channelCount: 2,
			},
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
				payloadType: 96,
				sps: []byte{
					0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
					0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
					0x00, 0x03, 0x00, 0x3d, 0x08,
				},
				pps: []byte{
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
				payloadType: 96,
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
				payloadType: 96,
				sps: []byte{
					0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50,
					0x05, 0xbb, 0x01, 0x6c, 0x80, 0x00, 0x00, 0x03,
					0x00, 0x80, 0x00, 0x00, 0x1e, 0x07, 0x8c, 0x18,
					0xcb,
				},
				pps: []byte{
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
				payloadType: 96,
				sps: []byte{
					0x67, 0x64, 0x00, 0x29, 0xac, 0x13, 0x31, 0x40,
					0x78, 0x04, 0x47, 0xde, 0x03, 0xea, 0x02, 0x02,
					0x03, 0xe0, 0x00, 0x00, 0x03, 0x00, 0x20, 0x00,
					0x00, 0x06, 0x52, 0x80,
				},
				pps: []byte{
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
			&TrackGeneric{
				clockRate: 90000,
				media:     "video",
				formats:   []string{"96"},
				rtpmap:    "96 H265/90000",
				fmtp: "96 sprop-vps=QAEMAf//AWAAAAMAkAAAAwAAAwB4mZgJ; " +
					"sprop-sps=QgEBAWAAAAMAkAAAAwAAAwB4oAPAgBDllmZpJMrgEAAAAwAQAAADAeCA; sprop-pps=RAHBcrRiQA==",
			},
		},
		{
			"multiple formats",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Port:    psdp.RangedPort{Value: 0},
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
				clockRate: 90000,
				media:     "video",
				formats:   []string{"98", "96"},
				rtpmap:    "98 H265/90000",
				fmtp: "98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
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
			"generic no formats",
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
			"generic no rtpmap",
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
			"generic invalid clockrate 1",
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
			"generic invalid clockrate 2",
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
			"generic invalid clockrate 3",
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
		baseURL *base.URL
		ur      *base.URL
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
			tracks, err := ReadTracks(ca.sdp, false)
			require.NoError(t, err)
			ur, err := tracks[0].url(ca.baseURL)
			require.NoError(t, err)
			require.Equal(t, ca.ur, ur)
		})
	}
}

func TestTrackURLError(t *testing.T) {
	track, err := NewTrackH264(96, []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x05, 0x06, 0x07, 0x08}, nil)
	require.NoError(t, err)
	_, err = track.url(nil)
	require.EqualError(t, err, "Content-Base header not provided")
}
