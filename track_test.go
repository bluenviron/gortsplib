package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

func TestTrackClockRate(t *testing.T) {
	for _, ca := range []struct {
		name      string
		sdp       []byte
		clockRate int
	}{
		{
			"empty encoding parameters",
			[]byte("v=0\r\n" +
				"o=- 38990265062388 38990265062388 IN IP4 192.168.1.142\r\n" +
				"s=RTSP Session\r\n" +
				"c=IN IP4 192.168.1.142\r\n" +
				"t=0 0\r\n" +
				"a=control:*\r\n" +
				"a=range:npt=0-\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000 \r\n" +
				"a=range:npt=0-\r\n" +
				"a=framerate:0S\r\n" +
				"a=fmtp:96 profile-level-id=64000c; packetization-mode=1; sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==\r\n" +
				"a=framerate:25\r\n" +
				"a=control:trackID=3\r\n"),
			90000,
		},
		{
			"static payload type 1",
			[]byte("v=0\r\n" +
				"o=- 38990265062388 38990265062388 IN IP4 192.168.1.142\r\n" +
				"s=RTSP Session\r\n" +
				"c=IN IP4 192.168.1.142\r\n" +
				"t=0 0\r\n" +
				"a=control:*\r\n" +
				"a=range:npt=0-\r\n" +
				"m=audio 0 RTP/AVP 8\r\n" +
				"a=control:trackID=4"),
			8000,
		},
		{
			"static payload type 2",
			[]byte("v=0\r\n" +
				"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
				"s=SDP Seminar\r\n" +
				"i=A Seminar on the session description protocol\r\n" +
				"u=http://www.example.com/seminars/sdp.pdf\r\n" +
				"e=j.doe@example.com (Jane Doe)\r\n" +
				"p=+1 617 555-6011\r\n" +
				"c=IN IP4 224.2.17.12/127\r\n" +
				"b=X-YZ:128\r\n" +
				"b=AS:12345\r\n" +
				"t=2873397496 2873404696\r\n" +
				"t=3034423619 3042462419\r\n" +
				"r=604800 3600 0 90000\r\n" +
				"z=2882844526 -3600 2898848070 0\r\n" +
				"k=prompt\r\n" +
				"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\r\n" +
				"a=recvonly\r\n" +
				"m=audio 49170 RTP/AVP 0\r\n" +
				"i=Vivamus a posuere nisl\r\n" +
				"c=IN IP4 203.0.113.1\r\n" +
				"b=X-YZ:128\r\n" +
				"k=prompt\r\n" +
				"a=sendrecv\r\n"),
			8000,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			tracks, err := ReadTracks(ca.sdp, nil)
			require.NoError(t, err)

			clockRate, err := tracks[0].ClockRate()
			require.NoError(t, err)

			require.Equal(t, clockRate, ca.clockRate)
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
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, url with special chars and query",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=5"),
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
			tracks, err := ReadTracks(ca.sdp, nil)
			require.NoError(t, err)
			tracks[0].BaseURL = ca.baseURL
			ur, err := tracks[0].URL()
			require.NoError(t, err)
			require.Equal(t, ca.ur, ur)
		})
	}
}

func TestTrackH264New(t *testing.T) {
	sps := []byte{
		0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
		0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
		0x00, 0x03, 0x00, 0x3d, 0x08,
	}

	pps := []byte{
		0x68, 0xee, 0x3c, 0x80,
	}

	tr, err := NewTrackH264(96, sps, pps)
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
					Key:   "fmtp",
					Value: "96 packetization-mode=1; sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C",
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
							Key:   "fmtp",
							Value: "96 packetization-mode=1; sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C",
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

func TestTrackH264Extract(t *testing.T) {
	for _, ca := range []struct {
		name  string
		track *Track
		sps   []byte
		pps   []byte
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
							Key:   "fmtp",
							Value: "96 packetization-mode=1; sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C",
						},
					},
				},
			},
			[]byte{
				0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
				0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
				0x00, 0x03, 0x00, 0x3d, 0x08,
			},
			[]byte{
				0x68, 0xee, 0x3c, 0x80,
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
							Key:   "fmtp",
							Value: "96 packetization-mode=1;profile-level-id=64001f;sprop-parameter-sets=Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,aOvjyyLA;",
						},
					},
				},
			},
			[]byte{
				0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50,
				0x05, 0xbb, 0x01, 0x6c, 0x80, 0x00, 0x00, 0x03,
				0x00, 0x80, 0x00, 0x00, 0x1e, 0x07, 0x8c, 0x18,
				0xcb,
			},
			[]byte{
				0x68, 0xeb, 0xe3, 0xcb, 0x22, 0xc0,
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			sps, pps, err := ca.track.ExtractDataH264()
			require.NoError(t, err)
			require.Equal(t, ca.sps, sps)
			require.Equal(t, ca.pps, pps)
		})
	}
}

func TestTrackH264ExtractErrors(t *testing.T) {
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
			_, _, err := ca.track.ExtractDataH264()
			require.Equal(t, ca.err, err.Error())
		})
	}
}

func TestTrackAACNew(t *testing.T) {
	tr, err := NewTrackAAC(96, []byte{17, 144})
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
	}, tr)
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

func TestTrackAACExtract(t *testing.T) {
	for _, ca := range []struct {
		name   string
		track  *Track
		config []byte
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
			[]byte{17, 144},
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
			[]byte{17, 144},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			config, err := ca.track.ExtractDataAAC()
			require.NoError(t, err)
			require.Equal(t, ca.config, config)
		})
	}
}

func TestTrackAACExtractErrors(t *testing.T) {
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
			"invalid config (96 profile-level-id=1; config=zz)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ca.track.ExtractDataAAC()
			require.Equal(t, ca.err, err.Error())
		})
	}
}
