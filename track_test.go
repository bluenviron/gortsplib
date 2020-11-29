package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
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
			tracks, err := ReadTracks(ca.sdp)
			require.NoError(t, err)

			clockRate, err := tracks[0].ClockRate()
			require.NoError(t, err)

			require.Equal(t, clockRate, ca.clockRate)
		})
	}
}
