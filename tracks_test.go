package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTracksReadErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		sdp  []byte
		err  string
	}{
		{
			"invalid SDP",
			[]byte{0x00, 0x01},
			"invalid line: (\x00\x01)",
		},
		{
			"invalid track",
			[]byte("v=0\r\n" +
				"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
				"s=SDP Seminar\r\n" +
				"m=video 0 RTP/AVP/TCP 96\r\n" +
				"a=rtpmap:96 H265/90000\r\n" +
				"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; " +
				"sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
				"a=control:streamid=0\r\n" +
				"m=audio 0 RTP/AVP/TCP 97\r\n" +
				"a=rtpmap:97 mpeg4-generic/44100/2\r\n" +
				"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=zzz1210\r\n" +
				"a=control:streamid=1\r\n"),
			"unable to parse track 2: invalid AAC config (zzz1210)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var tracks Tracks
			_, err := tracks.Unmarshal(ca.sdp, false)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestTracksReadSkipGenericTracksWithoutClockRate(t *testing.T) {
	sdp := []byte("v=0\r\n" +
		"o=- 0 0 IN IP4 10.0.0.131\r\n" +
		"s=Media Presentation\r\n" +
		"i=samsung\r\n" +
		"c=IN IP4 0.0.0.0\r\n" +
		"b=AS:2632\r\n" +
		"t=0 0\r\n" +
		"a=control:rtsp://10.0.100.50/profile5/media.smp\r\n" +
		"a=range:npt=now-\r\n" +
		"m=video 42504 RTP/AVP 97\r\n" +
		"b=AS:2560\r\n" +
		"a=rtpmap:97 H264/90000\r\n" +
		"a=control:rtsp://10.0.100.50/profile5/media.smp/trackID=v\r\n" +
		"a=cliprect:0,0,1080,1920\r\n" +
		"a=framesize:97 1920-1080\r\n" +
		"a=framerate:30.0\r\n" +
		"a=fmtp:97 packetization-mode=1;profile-level-id=640028;sprop-parameter-sets=Z2QAKKy0A8ARPyo=,aO4Bniw=\r\n" +
		"m=audio 42506 RTP/AVP 0\r\n" +
		"b=AS:64\r\n" +
		"a=rtpmap:0 PCMU/8000\r\n" +
		"a=control:rtsp://10.0.100.50/profile5/media.smp/trackID=a\r\n" +
		"a=recvonly\r\n" +
		"m=application 42508 RTP/AVP 107\r\n" +
		"b=AS:8\r\n")

	var tracks Tracks
	_, err := tracks.Unmarshal(sdp, true)
	require.NoError(t, err)
	require.Equal(t, Tracks{
		&TrackH264{
			trackBase: trackBase{
				control: "rtsp://10.0.100.50/profile5/media.smp/trackID=v",
			},
			PayloadType: 97,
			SPS:         []byte{0x67, 0x64, 0x00, 0x28, 0xac, 0xb4, 0x03, 0xc0, 0x11, 0x3f, 0x2a},
			PPS:         []byte{0x68, 0xee, 0x01, 0x9e, 0x2c},
		},
		&TrackPCMU{
			trackBase: trackBase{
				control: "rtsp://10.0.100.50/profile5/media.smp/trackID=a",
			},
		},
	}, tracks)
}
