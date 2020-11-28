package sdp

import (
	"net/url"
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

var cases = []struct {
	name string
	dec  []byte
	enc  []byte
	desc SessionDescription
}{
	// standard-compliant SDPs
	{
		"base",
		[]byte("v=0\r\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"i=A Seminar on the session description protocol\r\n" +
			"t=3034423619 3042462419\r\n"),
		[]byte("v=0\r\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"i=A Seminar on the session description protocol\r\n" +
			"t=3034423619 3042462419\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "jdoe",
				SessionID:      2890844526,
				SessionVersion: 2890842807,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.47.16.5",
			},
			SessionName: "SDP Seminar",
			SessionInformation: func() *psdp.Information {
				v := psdp.Information("A Seminar on the session description protocol")
				return &v
			}(),
			TimeDescriptions: []psdp.TimeDescription{
				{psdp.Timing{3034423619, 3042462419}, nil},
			},
		},
	},
	{
		"unix newlines",
		[]byte("v=0\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\n" +
			"s=SDP Seminar\n" +
			"i=A Seminar on the session description protocol\n" +
			"t=3034423619 3042462419\n"),
		[]byte("v=0\r\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"i=A Seminar on the session description protocol\r\n" +
			"t=3034423619 3042462419\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "jdoe",
				SessionID:      2890844526,
				SessionVersion: 2890842807,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.47.16.5",
			},
			SessionName: "SDP Seminar",
			SessionInformation: func() *psdp.Information {
				v := psdp.Information("A Seminar on the session description protocol")
				return &v
			}(),
			TimeDescriptions: []psdp.TimeDescription{
				{psdp.Timing{3034423619, 3042462419}, nil},
			},
		},
	},
	{
		"empty lines",
		[]byte("v=0\r\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
			"\r\n" +
			"s=SDP Seminar\r\n" +
			"\r\n" +
			"i=A Seminar on the session description protocol\r\n" +
			"\r\n" +
			"t=3034423619 3042462419\r\n" +
			"\r\n"),
		[]byte("v=0\r\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"i=A Seminar on the session description protocol\r\n" +
			"t=3034423619 3042462419\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "jdoe",
				SessionID:      2890844526,
				SessionVersion: 2890842807,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.47.16.5",
			},
			SessionName: "SDP Seminar",
			SessionInformation: func() *psdp.Information {
				v := psdp.Information("A Seminar on the session description protocol")
				return &v
			}(),
			TimeDescriptions: []psdp.TimeDescription{
				{psdp.Timing{3034423619, 3042462419}, nil},
			},
		},
	},
	{
		"full",
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
			"a=sendrecv\r\n" +
			"m=video 51372 RTP/AVP 99\r\n" +
			"a=rtpmap:99 h263-1998/90000\r\n"),
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
			"a=sendrecv\r\n" +
			"m=video 51372 RTP/AVP 99\r\n" +
			"a=rtpmap:99 h263-1998/90000\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "jdoe",
				SessionID:      2890844526,
				SessionVersion: 2890842807,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.47.16.5",
			},
			SessionName: "SDP Seminar",
			SessionInformation: func() *psdp.Information {
				v := psdp.Information("A Seminar on the session description protocol")
				return &v
			}(),
			URI: func() *url.URL {
				u, _ := url.Parse("http://www.example.com/seminars/sdp.pdf")
				return u
			}(),
			EmailAddress: func() *psdp.EmailAddress {
				v := psdp.EmailAddress("j.doe@example.com (Jane Doe)")
				return &v
			}(),
			PhoneNumber: func() *psdp.PhoneNumber {
				v := psdp.PhoneNumber("+1 617 555-6011")
				return &v
			}(),
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &psdp.Address{Address: "224.2.17.12/127"},
			},
			Bandwidth: []psdp.Bandwidth{
				{
					Experimental: true,
					Type:         "YZ",
					Bandwidth:    128,
				},
				{
					Type:      "AS",
					Bandwidth: 12345,
				},
			},
			TimeDescriptions: []psdp.TimeDescription{
				{psdp.Timing{2873397496, 2873404696}, nil},
				{psdp.Timing{3034423619, 3042462419}, []psdp.RepeatTime{{604800, 3600, []int64{0, 90000}}}},
			},
			TimeZones: []psdp.TimeZone{
				{2882844526, -3600},
				{2898848070, 0},
			},
			EncryptionKey: func() *psdp.EncryptionKey {
				v := psdp.EncryptionKey("prompt")
				return &v
			}(),
			Attributes: []psdp.Attribute{
				{"candidate", "0 1 UDP 2113667327 203.0.113.1 54400 typ host"},
				{"recvonly", ""},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Port:    psdp.RangedPort{Value: 49170},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"0"},
					},
					MediaTitle: func() *psdp.Information {
						v := psdp.Information("Vivamus a posuere nisl")
						return &v
					}(),
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address:     &psdp.Address{Address: "203.0.113.1"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Experimental: true,
							Type:         "YZ",
							Bandwidth:    128,
						},
					},
					EncryptionKey: func() *psdp.EncryptionKey {
						v := psdp.EncryptionKey("prompt")
						return &v
					}(),
					Attributes: []psdp.Attribute{
						{"sendrecv", ""},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 51372},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"99"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "99 h263-1998/90000"},
					},
				},
			},
		},
	},
	// non standard-compliant SDPs
	{
		"unordered global attributes",
		[]byte("v=0\r\n" +
			"o=- 1646532490 1646532490 IN IP4 10.175.31.17\r\n" +
			"a=control:*\r\n" +
			"a=source-filter: incl IN IP4 * 10.175.31.17\r\n" +
			"s=RTSP Server\r\n" +
			"a=range:npt=0-\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 profile-level-id=4D001E; packetization-mode=1; sprop-parameter-sets=Z00AHpWoKAv+VA==,aO48gA==\r\n" +
			"a=control:?ctype=video\r\n" +
			"a=recvonly\r\n" +
			"m=application 0 RTP/AVP 106\r\n" +
			"a=rtpmap:106 vnd.onvif.metadata/90000\r\n" +
			"a=control:?ctype=app106\r\n" +
			"a=sendonly\r\n"),
		[]byte("v=0\r\n" +
			"o=- 1646532490 1646532490 IN IP4 10.175.31.17\r\n" +
			"s=RTSP Server\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"a=source-filter: incl IN IP4 * 10.175.31.17\r\n" +
			"a=range:npt=0-\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 profile-level-id=4D001E; packetization-mode=1; sprop-parameter-sets=Z00AHpWoKAv+VA==,aO48gA==\r\n" +
			"a=control:?ctype=video\r\n" +
			"a=recvonly\r\n" +
			"m=application 0 RTP/AVP 106\r\n" +
			"a=rtpmap:106 vnd.onvif.metadata/90000\r\n" +
			"a=control:?ctype=app106\r\n" +
			"a=sendonly\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      1646532490,
				SessionVersion: 1646532490,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.175.31.17",
			},
			SessionName: "RTSP Server",
			TimeDescriptions: []psdp.TimeDescription{
				{psdp.Timing{0, 0}, nil},
			},
			Attributes: []psdp.Attribute{
				{"control", "*"},
				{"source-filter", " incl IN IP4 * 10.175.31.17"},
				{"range", "npt=0-"},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address:     &psdp.Address{Address: "0.0.0.0"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 H264/90000"},
						{"fmtp", "96 profile-level-id=4D001E; packetization-mode=1; sprop-parameter-sets=Z00AHpWoKAv+VA==,aO48gA=="},
						{"control", "?ctype=video"},
						{"recvonly", ""},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "application",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"106"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "106 vnd.onvif.metadata/90000"},
						{"control", "?ctype=app106"},
						{"sendonly", ""},
					},
				},
			},
		},
	},
	{
		"no timing",
		[]byte("v=0\r\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"m=video 0 RTP/AVP/TCP 96\r\n" +
			"a=rtpmap:96 H265/90000\r\n" +
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
			"a=control:streamid=0\r\n" +
			"m=audio 0 RTP/AVP/TCP 97\r\n" +
			"a=rtpmap:97 mpeg4-generic/44100/2\r\n" +
			"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210\r\n" +
			"a=control:streamid=1\r\n"),
		[]byte("v=0\r\n" +
			"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"m=video 0 RTP/AVP/TCP 96\r\n" +
			"a=rtpmap:96 H265/90000\r\n" +
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
			"a=control:streamid=0\r\n" +
			"m=audio 0 RTP/AVP/TCP 97\r\n" +
			"a=rtpmap:97 mpeg4-generic/44100/2\r\n" +
			"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210\r\n" +
			"a=control:streamid=1\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "jdoe",
				SessionID:      2890844526,
				SessionVersion: 2890842807,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.47.16.5",
			},
			SessionName: "SDP Seminar",
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP", "TCP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 H265/90000"},
						{"fmtp", "96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;"},
						{"control", "streamid=0"},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP", "TCP"},
						Formats: []string{"97"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "97 mpeg4-generic/44100/2"},
						{"fmtp", "97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210"},
						{"control", "streamid=1"},
					},
				},
			},
		},
	},
	{
		"no origin",
		[]byte("v=0\r\n" +
			"s=SDP Seminar\r\n" +
			"m=video 0 RTP/AVP/TCP 96\r\n" +
			"a=rtpmap:96 H265/90000\r\n" +
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
			"a=control:streamid=0\r\n" +
			"m=audio 0 RTP/AVP/TCP 97\r\n" +
			"a=rtpmap:97 mpeg4-generic/44100/2\r\n" +
			"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210\r\n" +
			"a=control:streamid=1\r\n"),
		[]byte("v=0\r\n" +
			"o= 0 0   \r\n" +
			"s=SDP Seminar\r\n" +
			"m=video 0 RTP/AVP/TCP 96\r\n" +
			"a=rtpmap:96 H265/90000\r\n" +
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
			"a=control:streamid=0\r\n" +
			"m=audio 0 RTP/AVP/TCP 97\r\n" +
			"a=rtpmap:97 mpeg4-generic/44100/2\r\n" +
			"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210\r\n" +
			"a=control:streamid=1\r\n"),
		SessionDescription{
			SessionName: "SDP Seminar",
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP", "TCP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 H265/90000"},
						{"fmtp", "96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;"},
						{"control", "streamid=0"},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP", "TCP"},
						Formats: []string{"97"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "97 mpeg4-generic/44100/2"},
						{"fmtp", "97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210"},
						{"control", "streamid=1"},
					},
				},
			},
		},
	},
	{
		"live reporter app",
		[]byte("v=0\r\n" +
			"o=-0 0 IN IP4 127.0.0.1\r\n" +
			"s=No Name\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=AS:253\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1; sprop-parameter-sets=J2QAHqxWgKA9pqAgIMBA,KO48sA==; profile-level-id=64001E\r\n" +
			"a=control:streamid=0\r\n" +
			"m=audio 0 RTP/AVP 97\r\n" +
			"b=AS:189\r\n" +
			"a=rtpmap:97 MPEG4-GENERIC/48000/1\r\n" +
			"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexLength=3;indexDeltaLength=3;config=118856E500\r\n" +
			"a=control:streamid=1\r\n"),
		[]byte("v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=No Name\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=AS:253\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1; sprop-parameter-sets=J2QAHqxWgKA9pqAgIMBA,KO48sA==; profile-level-id=64001E\r\n" +
			"a=control:streamid=0\r\n" +
			"m=audio 0 RTP/AVP 97\r\n" +
			"b=AS:189\r\n" +
			"a=rtpmap:97 MPEG4-GENERIC/48000/1\r\n" +
			"a=fmtp:97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexLength=3;indexDeltaLength=3;config=118856E500\r\n" +
			"a=control:streamid=1\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      0,
				SessionVersion: 0,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "127.0.0.1",
			},
			SessionName: "No Name",
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &psdp.Address{Address: "0.0.0.0"},
			},
			TimeDescriptions: []psdp.TimeDescription{{psdp.Timing{0, 0}, nil}},
			Attributes: []psdp.Attribute{
				{"control", "*"},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "AS",
							Bandwidth: 253,
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 H264/90000"},
						{"fmtp", "96 packetization-mode=1; sprop-parameter-sets=J2QAHqxWgKA9pqAgIMBA,KO48sA==; profile-level-id=64001E"},
						{"control", "streamid=0"},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"97"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "AS",
							Bandwidth: 189,
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "97 MPEG4-GENERIC/48000/1"},
						{"fmtp", "97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexLength=3;indexDeltaLength=3;config=118856E500"},
						{"control", "streamid=1"},
					},
				},
			},
		},
	},
	{
		"sony_snc_wr630",
		[]byte("v=0\r\n" +
			"o=- 12345 1 IN IP4 10.21.61.139\r\n" +
			"s=Sony RTSP Server\r\n" +
			"t=0 0\r\n" +
			"a=range:npt=now-\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"m=video 0 RTP/AVP 105\r\n" +
			"a=rtpmap:105 H264/90000\r\n" +
			"a=control:trackID=1\r\n" +
			"a=recvonly\r\n" +
			"a=framerate:25.0\r\n" +
			"a=fmtp:105 packetization-mode=1; profile-level-id=640028; sprop-parameter-sets=Z2QAKKwa0A8ARPy4CIAAAAMAgAAADLWgAtwAHJ173CPFCKg=,KO4ESSJAAAAAAAAAAA==\r\n"),
		[]byte("v=0\r\n" +
			"o=- 12345 1 IN IP4 10.21.61.139\r\n" +
			"s=Sony RTSP Server\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=range:npt=now-\r\n" +
			"m=video 0 RTP/AVP 105\r\n" +
			"a=rtpmap:105 H264/90000\r\n" +
			"a=control:trackID=1\r\n" +
			"a=recvonly\r\n" +
			"a=framerate:25.0\r\n" +
			"a=fmtp:105 packetization-mode=1; profile-level-id=640028; sprop-parameter-sets=Z2QAKKwa0A8ARPy4CIAAAAMAgAAADLWgAtwAHJ173CPFCKg=,KO4ESSJAAAAAAAAAAA==\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      12345,
				SessionVersion: 1,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.21.61.139",
			},
			SessionName: psdp.SessionName("Sony RTSP Server"),
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &psdp.Address{Address: "0.0.0.0"},
			},
			TimeDescriptions: []psdp.TimeDescription{{psdp.Timing{0, 0}, nil}},
			Attributes: []psdp.Attribute{
				{"range", "npt=now-"},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"105"},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "105 H264/90000"},
						{"control", "trackID=1"},
						{"recvonly", ""},
						{"framerate", "25.0"},
						{"fmtp", "105 packetization-mode=1; profile-level-id=640028; sprop-parameter-sets=Z2QAKKwa0A8ARPy4CIAAAAMAgAAADLWgAtwAHJ173CPFCKg=,KO4ESSJAAAAAAAAAAA=="},
					},
				},
			},
		},
	},
	{
		"vlc",
		[]byte("v=0\r\n" +
			"o=- 16379793953309178445 16379793953309178445 IN IP4 5c2b68da\r\n" +
			"s=Unnamed\r\n" +
			"i=N/A\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=tool:vlc 3.0.11\r\n" +
			"a=recvonly\r\n" +
			"a=type:broadcast\r\n" +
			"a=charset:UTF-8\r\n" +
			"m=audio 0 RTP/AVP 96\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 mpeg4-generic/22050\r\n" +
			"a=fmtp:96 streamtype=5; profile-level-id=15; mode=AAC-hbr; config=1388; SizeLength=13; IndexLength=3; IndexDeltaLength=3; Profile=1;\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=640028;sprop-parameter-sets=J2QAKKwrQCgDzQDxImo=,KO4CXLA=;\r\n"),
		[]byte("v=0\r\n" +
			"o=- 16379793953309178445 16379793953309178445 IN IP4 5c2b68da\r\n" +
			"s=Unnamed\r\n" +
			"i=N/A\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=tool:vlc 3.0.11\r\n" +
			"a=recvonly\r\n" +
			"a=type:broadcast\r\n" +
			"a=charset:UTF-8\r\n" +
			"m=audio 0 RTP/AVP 96\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 mpeg4-generic/22050\r\n" +
			"a=fmtp:96 streamtype=5; profile-level-id=15; mode=AAC-hbr; config=1388; SizeLength=13; IndexLength=3; IndexDeltaLength=3; Profile=1;\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=640028;sprop-parameter-sets=J2QAKKwrQCgDzQDxImo=,KO4CXLA=;\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      16379793953309178445,
				SessionVersion: 16379793953309178445,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "5c2b68da",
			},
			SessionName: psdp.SessionName("Unnamed"),
			SessionInformation: func() *psdp.Information {
				v := psdp.Information("N/A")
				return &v
			}(),
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &psdp.Address{Address: "0.0.0.0"},
			},
			TimeDescriptions: []psdp.TimeDescription{{psdp.Timing{0, 0}, nil}},
			Attributes: []psdp.Attribute{
				{"tool", "vlc 3.0.11"},
				{"recvonly", ""},
				{"type", "broadcast"},
				{"charset", "UTF-8"},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type: "RR",
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 mpeg4-generic/22050"},
						{"fmtp", "96 streamtype=5; profile-level-id=15; mode=AAC-hbr; config=1388; SizeLength=13; IndexLength=3; IndexDeltaLength=3; Profile=1;"},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type: "RR",
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 H264/90000"},
						{"fmtp", "96 packetization-mode=1;profile-level-id=640028;sprop-parameter-sets=J2QAKKwrQCgDzQDxImo=,KO4CXLA=;"},
					},
				},
			},
		},
	},
	{
		"mp2t",
		[]byte("v=0\r\n" +
			"o=- 1702415089 4281335390 IN IP4 127.0.0.1\r\n" +
			"s=live\r\n" +
			"t=0 0\r\n" +
			"c=IN IP4 239.3.1.142\r\n" +
			"a=range:clock=0-\r\n" +
			"m=video 8048 MP2T/AVP 33\r\n" +
			"b=AS:7655\r\n"),
		[]byte("v=0\r\n" +
			"o=- 1702415089 4281335390 IN IP4 127.0.0.1\r\n" +
			"s=live\r\n" +
			"c=IN IP4 239.3.1.142\r\n" +
			"t=0 0\r\n" +
			"a=range:clock=0-\r\n" +
			"m=video 8048 MP2T/AVP 33\r\n" +
			"b=AS:7655\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      1702415089,
				SessionVersion: 4281335390,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "127.0.0.1",
			},
			SessionName: psdp.SessionName("live"),
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &psdp.Address{Address: "239.3.1.142"},
			},
			TimeDescriptions: []psdp.TimeDescription{{psdp.Timing{0, 0}, nil}},
			Attributes: []psdp.Attribute{
				{"range", "clock=0-"},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 8048},
						Protos:  []string{"MP2T", "AVP"},
						Formats: []string{"33"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "AS",
							Bandwidth: 7655,
						},
					},
				},
			},
		},
	},
	{
		"empty unicast address in origin",
		[]byte("v=0\r\n" +
			"o=RTSP 16381778200090761968 16381778200090839277 IN IP4 \r\n" +
			"s=RTSP Server\r\n" +
			"e=NONE\r\n" +
			"t=0 0\r\n" +
			"a=recvonly\r\n" +
			"a=x-dimensions:1920,1080\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=64001e;sprop-parameter-sets=Z2QAHqwsaoMg5puAgICB,aO4xshs=\r\n" +
			"a=Media_header:MEDIAINFO=494D4B48010100000400010000000000000000000000000000000000000000000000000000000000;\r\n" +
			"a=appversion:1.0\r\n" +
			"b=AS:5000\r\n" +
			"a=control:rtsp://10.10.1.30:8554/onvif2/audio/trackID=0\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtpmap:0 PCMU/8000/1\r\n" +
			"b=AS:5000\r\n" +
			"a=control:rtsp://10.10.1.30:8554/onvif2/audio/trackID=1\r\n"),
		[]byte("v=0\r\n" +
			"o=RTSP 16381778200090761968 16381778200090839277 IN IP4 127.0.0.1\r\n" +
			"s=RTSP Server\r\n" +
			"e=NONE\r\n" +
			"t=0 0\r\n" +
			"a=recvonly\r\n" +
			"a=x-dimensions:1920,1080\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:5000\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=64001e;sprop-parameter-sets=Z2QAHqwsaoMg5puAgICB,aO4xshs=\r\n" +
			"a=Media_header:MEDIAINFO=494D4B48010100000400010000000000000000000000000000000000000000000000000000000000;\r\n" +
			"a=appversion:1.0\r\n" +
			"a=control:rtsp://10.10.1.30:8554/onvif2/audio/trackID=0\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:5000\r\n" +
			"a=rtpmap:0 PCMU/8000/1\r\n" +
			"a=control:rtsp://10.10.1.30:8554/onvif2/audio/trackID=1\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "RTSP",
				SessionID:      16381778200090761968,
				SessionVersion: 16381778200090839277,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "127.0.0.1",
			},
			SessionName: psdp.SessionName("RTSP Server"),
			EmailAddress: func() *psdp.EmailAddress {
				v := psdp.EmailAddress("NONE")
				return &v
			}(),
			TimeDescriptions: []psdp.TimeDescription{{psdp.Timing{0, 0}, nil}},
			Attributes: []psdp.Attribute{
				{"recvonly", ""},
				{"x-dimensions", "1920,1080"},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address:     &psdp.Address{Address: "0.0.0.0"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "AS",
							Bandwidth: 5000,
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 H264/90000"},
						{"fmtp", "96 packetization-mode=1;profile-level-id=64001e;sprop-parameter-sets=Z2QAHqwsaoMg5puAgICB,aO4xshs="},
						{"Media_header", "MEDIAINFO=494D4B48010100000400010000000000000000000000000000000000000000000000000000000000;"},
						{"appversion", "1.0"},
						{"control", "rtsp://10.10.1.30:8554/onvif2/audio/trackID=0"},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"0"},
					},
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address:     &psdp.Address{Address: "0.0.0.0"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "AS",
							Bandwidth: 5000,
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "0 PCMU/8000/1"},
						{"control", "rtsp://10.10.1.30:8554/onvif2/audio/trackID=1"},
					},
				},
			},
		},
	},
	{
		"bandwidth rs",
		[]byte("v=0\r\n" +
			"o=- 200710060441230578 200710060441230578 IN IP4 127.0.0.1\r\n" +
			"s=<No Title>\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:104\r\n" +
			"a=maxps:1250\r\n" +
			"t=0 0\r\n" +
			"a=control:rtsp://61.135.88.175:554/refuse/unavailable_media.wmv/\r\n" +
			"a=etag:{CCEE392D-83DF-F4AA-130B-E8A05562CE63}\r\n" +
			"a=range:npt=3.000-6.185\r\n" +
			"a=type:notstridable\r\n" +
			"a=recvonly\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=AS:105\r\n" +
			"b=X-AV:100\r\n" +
			"b=RS:0\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 x-asf-pf/1000\r\n" +
			"a=control:video\r\n" +
			"a=stream:1\r\n" +
			"m=application 0 RTP/AVP 96\r\n" +
			"b=RS:0\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 x-wms-rtx/1000\r\n" +
			"a=control:rtx\r\n" +
			"a=stream:65536\r\n"),
		[]byte("v=0\r\n" +
			"o=- 200710060441230578 200710060441230578 IN IP4 127.0.0.1\r\n" +
			"s=<No Title>\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:104\r\n" +
			"t=0 0\r\n" +
			"a=maxps:1250\r\n" +
			"a=control:rtsp://61.135.88.175:554/refuse/unavailable_media.wmv/\r\n" +
			"a=etag:{CCEE392D-83DF-F4AA-130B-E8A05562CE63}\r\n" +
			"a=range:npt=3.000-6.185\r\n" +
			"a=type:notstridable\r\n" +
			"a=recvonly\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=AS:105\r\n" +
			"b=X-AV:100\r\n" +
			"b=RS:0\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 x-asf-pf/1000\r\n" +
			"a=control:video\r\n" +
			"a=stream:1\r\n" +
			"m=application 0 RTP/AVP 96\r\n" +
			"b=RS:0\r\n" +
			"b=RR:0\r\n" +
			"a=rtpmap:96 x-wms-rtx/1000\r\n" +
			"a=control:rtx\r\n" +
			"a=stream:65536\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      200710060441230578,
				SessionVersion: 200710060441230578,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "127.0.0.1",
			},
			SessionName: psdp.SessionName("<No Title>"),
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address:     &psdp.Address{Address: "0.0.0.0"},
			},
			Bandwidth: []psdp.Bandwidth{
				{
					Type:      "AS",
					Bandwidth: 104,
				},
			},
			TimeDescriptions: []psdp.TimeDescription{{psdp.Timing{0, 0}, nil}},
			Attributes: []psdp.Attribute{
				{"maxps", "1250"},
				{"control", "rtsp://61.135.88.175:554/refuse/unavailable_media.wmv/"},
				{"etag", "{CCEE392D-83DF-F4AA-130B-E8A05562CE63}"},
				{"range", "npt=3.000-6.185"},
				{"type", "notstridable"},
				{"recvonly", ""},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "AS",
							Bandwidth: 105,
						},
						{
							Experimental: true,
							Type:         "AV",
							Bandwidth:    100,
						},
						{
							Type:      "RS",
							Bandwidth: 0,
						},
						{
							Type:      "RR",
							Bandwidth: 0,
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 x-asf-pf/1000"},
						{"control", "video"},
						{"stream", "1"},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "application",
						Port:    psdp.RangedPort{Value: 0},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "RS",
							Bandwidth: 0,
						},
						{
							Type:      "RR",
							Bandwidth: 0,
						},
					},
					Attributes: []psdp.Attribute{
						{"rtpmap", "96 x-wms-rtx/1000"},
						{"control", "rtx"},
						{"stream", "65536"},
					},
				},
			},
		},
	},
}

func TestUnmarshal(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			desc := SessionDescription{}
			err := desc.Unmarshal(c.dec)
			require.NoError(t, err)
			require.Equal(t, c.desc, desc)
		})
	}
}

func TestMarshal(t *testing.T) {
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enc, err := c.desc.Marshal()
			require.NoError(t, err)
			require.Equal(t, string(c.enc), string(enc))
		})
	}
}
