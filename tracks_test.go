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
			_, err := ReadTracks(ca.sdp)
			require.EqualError(t, err, ca.err)
		})
	}
}
