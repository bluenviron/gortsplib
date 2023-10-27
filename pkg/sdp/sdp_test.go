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
	// compliant SDPs
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
				{Timing: psdp.Timing{StartTime: 3034423619, StopTime: 3042462419}},
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
				{Timing: psdp.Timing{StartTime: 3034423619, StopTime: 3042462419}},
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
				{Timing: psdp.Timing{StartTime: 3034423619, StopTime: 3042462419}},
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
				{Timing: psdp.Timing{StartTime: 2873397496, StopTime: 2873404696}},
				{
					Timing:      psdp.Timing{StartTime: 3034423619, StopTime: 3042462419},
					RepeatTimes: []psdp.RepeatTime{{Interval: 604800, Duration: 3600, Offsets: []int64{0, 90000}}},
				},
			},
			TimeZones: []psdp.TimeZone{
				{AdjustmentTime: 2882844526, Offset: -3600},
				{AdjustmentTime: 2898848070},
			},
			EncryptionKey: func() *psdp.EncryptionKey {
				v := psdp.EncryptionKey("prompt")
				return &v
			}(),
			Attributes: []psdp.Attribute{
				{
					Key:   "candidate",
					Value: "0 1 UDP 2113667327 203.0.113.1 54400 typ host",
				},
				{
					Key:   "recvonly",
					Value: "",
				},
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
						{
							Key: "sendrecv",
						},
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
						{
							Key:   "rtpmap",
							Value: "99 h263-1998/90000",
						},
					},
				},
			},
		},
	},
	// non compliant SDPs
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
				{Timing: psdp.Timing{StartTime: 0, StopTime: 0}},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "control",
					Value: "*",
				},
				{
					Key:   "source-filter",
					Value: " incl IN IP4 * 10.175.31.17",
				},
				{
					Key:   "range",
					Value: "npt=0-",
				},
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
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=4D001E; packetization-mode=1; sprop-parameter-sets=Z00AHpWoKAv+VA==,aO48gA==",
						},
						{
							Key:   "control",
							Value: "?ctype=video",
						},
						{
							Key:   "recvonly",
							Value: "",
						},
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
						{
							Key:   "rtpmap",
							Value: "106 vnd.onvif.metadata/90000",
						},
						{
							Key:   "control",
							Value: "?ctype=app106",
						},
						{
							Key:   "sendonly",
							Value: "",
						},
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
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; " +
			"sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
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
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; " +
			"sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
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
						{
							Key:   "rtpmap",
							Value: "96 H265/90000",
						},
						{
							Key: "fmtp",
							Value: "96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; " +
								"sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;",
						},
						{
							Key:   "control",
							Value: "streamid=0",
						},
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
						{
							Key:   "rtpmap",
							Value: "97 mpeg4-generic/44100/2",
						},
						{
							Key:   "fmtp",
							Value: "97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210",
						},
						{
							Key:   "control",
							Value: "streamid=1",
						},
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
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; " +
			"sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
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
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; " +
			"sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;\r\n" +
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
						{
							Key:   "rtpmap",
							Value: "96 H265/90000",
						},
						{
							Key: "fmtp",
							Value: "96 sprop-vps=QAEMAf//AWAAAAMAsAAAAwAAAwB4FwJA; " +
								"sprop-sps=QgEBAWAAAAMAsAAAAwAAAwB4oAKggC8c1YgXuRZFL/y5/E/qbgQEBAE=; sprop-pps=RAHAcvBTJA==;",
						},
						{
							Key:   "control",
							Value: "streamid=0",
						},
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
						{
							Key:   "rtpmap",
							Value: "97 mpeg4-generic/44100/2",
						},
						{
							Key:   "fmtp",
							Value: "97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1210",
						},
						{
							Key:   "control",
							Value: "streamid=1",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/75",
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
			TimeDescriptions: []psdp.TimeDescription{{Timing: psdp.Timing{StartTime: 0, StopTime: 0}}},
			Attributes: []psdp.Attribute{
				{
					Key:   "control",
					Value: "*",
				},
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
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 packetization-mode=1; sprop-parameter-sets=J2QAHqxWgKA9pqAgIMBA,KO48sA==; profile-level-id=64001E",
						},
						{
							Key:   "control",
							Value: "streamid=0",
						},
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
						{
							Key:   "rtpmap",
							Value: "97 MPEG4-GENERIC/48000/1",
						},
						{
							Key:   "fmtp",
							Value: "97 profile-level-id=1;mode=AAC-hbr;sizelength=13;indexLength=3;indexDeltaLength=3;config=118856E500",
						},
						{
							Key:   "control",
							Value: "streamid=1",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/85",
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
			"a=fmtp:105 packetization-mode=1; profile-level-id=640028; " +
			"sprop-parameter-sets=Z2QAKKwa0A8ARPy4CIAAAAMAgAAADLWgAtwAHJ173CPFCKg=,KO4ESSJAAAAAAAAAAA==\r\n"),
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
			"a=fmtp:105 packetization-mode=1; profile-level-id=640028; " +
			"sprop-parameter-sets=Z2QAKKwa0A8ARPy4CIAAAAMAgAAADLWgAtwAHJ173CPFCKg=,KO4ESSJAAAAAAAAAAA==\r\n"),
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
			TimeDescriptions: []psdp.TimeDescription{{Timing: psdp.Timing{StartTime: 0, StopTime: 0}}},
			Attributes: []psdp.Attribute{
				{
					Key:   "range",
					Value: "npt=now-",
				},
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
						{
							Key:   "rtpmap",
							Value: "105 H264/90000",
						},
						{
							Key:   "control",
							Value: "trackID=1",
						},
						{
							Key:   "recvonly",
							Value: "",
						},
						{
							Key:   "framerate",
							Value: "25.0",
						},
						{
							Key: "fmtp",
							Value: "105 packetization-mode=1; profile-level-id=640028; " +
								"sprop-parameter-sets=Z2QAKKwa0A8ARPy4CIAAAAMAgAAADLWgAtwAHJ173CPFCKg=,KO4ESSJAAAAAAAAAAA==",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/115",
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
			"a=fmtp:96 streamtype=5; profile-level-id=15; mode=AAC-hbr; " +
			"config=1388; SizeLength=13; IndexLength=3; IndexDeltaLength=3; Profile=1;\r\n" +
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
			"a=fmtp:96 streamtype=5; profile-level-id=15; mode=AAC-hbr; " +
			"config=1388; SizeLength=13; IndexLength=3; IndexDeltaLength=3; Profile=1;\r\n" +
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
			TimeDescriptions: []psdp.TimeDescription{{Timing: psdp.Timing{StartTime: 0, StopTime: 0}}},
			Attributes: []psdp.Attribute{
				{
					Key:   "tool",
					Value: "vlc 3.0.11",
				},
				{Key: "recvonly"},
				{
					Key:   "type",
					Value: "broadcast",
				},
				{
					Key:   "charset",
					Value: "UTF-8",
				},
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
						{
							Key:   "rtpmap",
							Value: "96 mpeg4-generic/22050",
						},
						{
							Key: "fmtp",
							Value: "96 streamtype=5; profile-level-id=15; " +
								"mode=AAC-hbr; config=1388; SizeLength=13; IndexLength=3; IndexDeltaLength=3; Profile=1;",
						},
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
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 packetization-mode=1;profile-level-id=640028;sprop-parameter-sets=J2QAKKwrQCgDzQDxImo=,KO4CXLA=;",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/120",
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
			TimeDescriptions: []psdp.TimeDescription{{Timing: psdp.Timing{StartTime: 0, StopTime: 0}}},
			Attributes: []psdp.Attribute{
				{
					Key:   "range",
					Value: "clock=0-",
				},
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
		"issue mediamtx/121",
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
			"o=RTSP 16381778200090761968 16381778200090839277 IN IP4 \r\n" +
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
				UnicastAddress: "",
			},
			SessionName: psdp.SessionName("RTSP Server"),
			EmailAddress: func() *psdp.EmailAddress {
				v := psdp.EmailAddress("NONE")
				return &v
			}(),
			TimeDescriptions: []psdp.TimeDescription{{Timing: psdp.Timing{StartTime: 0, StopTime: 0}}},
			Attributes: []psdp.Attribute{
				{Key: "recvonly"},
				{
					Key:   "x-dimensions",
					Value: "1920,1080",
				},
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
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "fmtp",
							Value: "96 packetization-mode=1;profile-level-id=64001e;sprop-parameter-sets=Z2QAHqwsaoMg5puAgICB,aO4xshs=",
						},
						{
							Key:   "Media_header",
							Value: "MEDIAINFO=494D4B48010100000400010000000000000000000000000000000000000000000000000000000000;",
						},
						{
							Key:   "appversion",
							Value: "1.0",
						},
						{
							Key:   "control",
							Value: "rtsp://10.10.1.30:8554/onvif2/audio/trackID=0",
						},
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
						{
							Key:   "rtpmap",
							Value: "0 PCMU/8000/1",
						},
						{
							Key:   "control",
							Value: "rtsp://10.10.1.30:8554/onvif2/audio/trackID=1",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/136",
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
			TimeDescriptions: []psdp.TimeDescription{{Timing: psdp.Timing{StartTime: 0, StopTime: 0}}},
			Attributes: []psdp.Attribute{
				{
					Key:   "maxps",
					Value: "1250",
				},
				{
					Key:   "control",
					Value: "rtsp://61.135.88.175:554/refuse/unavailable_media.wmv/",
				},
				{
					Key:   "etag",
					Value: "{CCEE392D-83DF-F4AA-130B-E8A05562CE63}",
				},
				{
					Key:   "range",
					Value: "npt=3.000-6.185",
				},
				{
					Key:   "type",
					Value: "notstridable",
				},
				{Key: "recvonly"},
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
						{
							Key:   "rtpmap",
							Value: "96 x-asf-pf/1000",
						},
						{
							Key:   "control",
							Value: "video",
						},
						{
							Key:   "stream",
							Value: "1",
						},
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
						{
							Key:   "rtpmap",
							Value: "96 x-wms-rtx/1000",
						},
						{
							Key:   "control",
							Value: "rtx",
						},
						{
							Key:   "stream",
							Value: "65536",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/127",
		[]byte("v=0\r\n" +
			"o=RTSP Session 1 2 IN IP4 0.0.0.0\r\n" +
			"s=Sony RTSP Server\r\n"),
		[]byte("v=0\r\n" +
			"o=RTSP Session 1 2 IN IP4 0.0.0.0\r\n" +
			"s=Sony RTSP Server\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "RTSP Session",
				SessionID:      1,
				SessionVersion: 2,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "0.0.0.0",
			},
			SessionName: psdp.SessionName("Sony RTSP Server"),
		},
	},
	{
		"issue mediamtx/227",
		[]byte("v=0\r\n" +
			"o=- 1109162014219182 0 IN IP4 0.0.0.0\r\n" +
			"s=HIK Media Server V3.1.3\r\n" +
			"i=HIK Media Server Session Description : standard\r\n" +
			"e=NONE\r\n" +
			"c=IN c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"a=range:npt=now-\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"i=Video Media\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 profile-level-id=4D0014;packetization-mode=0;" +
			"sprop-parameter-sets=Z01AHppmBYHv81BgYGQAAA+gAAF3ABA=,aO48gA==\r\n" +
			"a=control:trackID=video\r\n" +
			"a=Media_header:MEDIAINFO=494D4B48010100000400000100000000000000000000000000000000000000000000000000000000;\r\n" +
			"a=appversion:1.0\r\n"),
		[]byte("v=0\r\n" +
			"o=- 1109162014219182 0 IN IP4 0.0.0.0\r\n" +
			"s=HIK Media Server V3.1.3\r\n" +
			"i=HIK Media Server Session Description : standard\r\n" +
			"e=NONE\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"a=range:npt=now-\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"i=Video Media\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 profile-level-id=4D0014;packetization-mode=0;" +
			"sprop-parameter-sets=Z01AHppmBYHv81BgYGQAAA+gAAF3ABA=,aO48gA==\r\n" +
			"a=control:trackID=video\r\n" +
			"a=Media_header:MEDIAINFO=494D4B48010100000400000100000000000000000000000000000000000000000000000000000000;\r\n" +
			"a=appversion:1.0\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      1109162014219182,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "0.0.0.0",
			},
			SessionName: psdp.SessionName("HIK Media Server V3.1.3"),
			SessionInformation: func() *psdp.Information {
				v := psdp.Information("HIK Media Server Session Description : standard")
				return &v
			}(),
			EmailAddress: func() *psdp.EmailAddress {
				v := psdp.EmailAddress("NONE")
				return &v
			}(),
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address: &psdp.Address{
					Address: "0.0.0.0",
				},
			},
			TimeDescriptions: []psdp.TimeDescription{
				{},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "control",
					Value: "*",
				},
				{
					Key:   "range",
					Value: "npt=now-",
				},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					MediaTitle: func() *psdp.Information {
						v := psdp.Information("Video Media")
						return &v
					}(),
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key: "fmtp",
							Value: "96 profile-level-id=4D0014;packetization-mode=0;" +
								"sprop-parameter-sets=Z01AHppmBYHv81BgYGQAAA+gAAF3ABA=,aO48gA==",
						},
						{
							Key:   "control",
							Value: "trackID=video",
						},
						{
							Key: "Media_header",
							Value: "MEDIAINFO=494D4B480101000004000001000000000000000000000000" +
								"00000000000000000000000000000000;",
						},
						{
							Key:   "appversion",
							Value: "1.0",
						},
					},
				},
			},
		},
	},
	{
		"issue gortsplib/60",
		[]byte("v=0\r\n" +
			"o=jdoe 0xAC4EC96E 2890842807 IN IP4 10.47.16.5\r\n" +
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
				{Timing: psdp.Timing{StartTime: 3034423619, StopTime: 3042462419}},
			},
		},
	},
	{
		"issue gortsplib/85",
		[]byte("v=0\r\n" +
			"o=- 0 0 IN IP4 172.16.2.20\r\n" +
			"s=IR stream\r\n" +
			"i=Live infrared\r\n" +
			"c=IN IP4 172.16.2.20\r\n" +
			"t=now-\r\n" +
			"m=video 0 RTP/AVP 96 97 111 112 99\r\n" +
			"a=control:rtsp://172.16.2.20/sid=96&overlay=on\r\n" +
			"a=framerate:30\r\n" +
			"a=rtpmap:96 MP4V-ES/90000\r\n" +
			"a=framesize:96 640-480\r\n" +
			"a=fmtp:96 profile-level-id=1;config=000001B002000001B59113000001000000012000C888800F514043C14103\r\n" +
			"a=rtpmap:97 MP4V-ES/90000\r\n" +
			"a=framesize:97 320-240\r\n" +
			"a=fmtp:97 profile-level-id=1;config=000001B002000001B59113000001000000012000C888800F50A041E14103\r\n" +
			"a=rtpmap:111 H264/90000\r\n" +
			"a=framesize:111 640-480\r\n" +
			"a=fmtp:111 profile-level-id=42001E;packetization-mode=1;sprop-parameter-sets=Z0IAHqtAUB7I,aM4xEg==\r\n" +
			"a=rtpmap:112 H264/90000\r\n" +
			"a=framesize:112 320-240\r\n" +
			"a=fmtp:112 profile-level-id=42001E;packetization-mode=1;sprop-parameter-sets=Z0IAHqtAoPyA,aM4xEg==\r\n" +
			"a=rtpmap:99 FCAM/90000\r\n" +
			"a=framesize:99 320-240\r\n" +
			"a=fmtp:99 sampling=mono; width=320; height=240; depth=16\r\n"),
		[]byte("v=0\r\n" +
			"o=- 0 0 IN IP4 172.16.2.20\r\n" +
			"s=IR stream\r\n" +
			"i=Live infrared\r\n" +
			"c=IN IP4 172.16.2.20\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96 97 111 112 99\r\n" +
			"a=control:rtsp://172.16.2.20/sid=96&overlay=on\r\n" +
			"a=framerate:30\r\n" +
			"a=rtpmap:96 MP4V-ES/90000\r\n" +
			"a=framesize:96 640-480\r\n" +
			"a=fmtp:96 profile-level-id=1;config=000001B002000001B59113000001000000012000C888800F514043C14103\r\n" +
			"a=rtpmap:97 MP4V-ES/90000\r\n" +
			"a=framesize:97 320-240\r\n" +
			"a=fmtp:97 profile-level-id=1;config=000001B002000001B59113000001000000012000C888800F50A041E14103\r\n" +
			"a=rtpmap:111 H264/90000\r\n" +
			"a=framesize:111 640-480\r\n" +
			"a=fmtp:111 profile-level-id=42001E;packetization-mode=1;sprop-parameter-sets=Z0IAHqtAUB7I,aM4xEg==\r\n" +
			"a=rtpmap:112 H264/90000\r\n" +
			"a=framesize:112 320-240\r\n" +
			"a=fmtp:112 profile-level-id=42001E;packetization-mode=1;sprop-parameter-sets=Z0IAHqtAoPyA,aM4xEg==\r\n" +
			"a=rtpmap:99 FCAM/90000\r\n" +
			"a=framesize:99 320-240\r\n" +
			"a=fmtp:99 sampling=mono; width=320; height=240; depth=16\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      0,
				SessionVersion: 0,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "172.16.2.20",
			},
			SessionName: "IR stream",
			SessionInformation: func() *psdp.Information {
				v := psdp.Information("Live infrared")
				return &v
			}(),
			TimeDescriptions: []psdp.TimeDescription{
				{Timing: psdp.Timing{StartTime: 0, StopTime: 0}},
			},
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address: &psdp.Address{
					Address: "172.16.2.20",
				},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96", "97", "111", "112", "99"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "control",
							Value: "rtsp://172.16.2.20/sid=96&overlay=on",
						},
						{
							Key:   "framerate",
							Value: "30",
						},
						{
							Key:   "rtpmap",
							Value: "96 MP4V-ES/90000",
						},
						{
							Key:   "framesize",
							Value: "96 640-480",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=1;config=000001B002000001B59113000001000000012000C888800F514043C14103",
						},
						{
							Key:   "rtpmap",
							Value: "97 MP4V-ES/90000",
						},
						{
							Key:   "framesize",
							Value: "97 320-240",
						},
						{
							Key:   "fmtp",
							Value: "97 profile-level-id=1;config=000001B002000001B59113000001000000012000C888800F50A041E14103",
						},
						{
							Key:   "rtpmap",
							Value: "111 H264/90000",
						},
						{
							Key:   "framesize",
							Value: "111 640-480",
						},
						{
							Key:   "fmtp",
							Value: "111 profile-level-id=42001E;packetization-mode=1;sprop-parameter-sets=Z0IAHqtAUB7I,aM4xEg==",
						},
						{
							Key:   "rtpmap",
							Value: "112 H264/90000",
						},
						{
							Key:   "framesize",
							Value: "112 320-240",
						},
						{
							Key:   "fmtp",
							Value: "112 profile-level-id=42001E;packetization-mode=1;sprop-parameter-sets=Z0IAHqtAoPyA,aM4xEg==",
						},
						{
							Key:   "rtpmap",
							Value: "99 FCAM/90000",
						},
						{
							Key:   "framesize",
							Value: "99 320-240",
						},
						{
							Key:   "fmtp",
							Value: "99 sampling=mono; width=320; height=240; depth=16",
						},
					},
				},
			},
		},
	},
	{
		"issue gortsplib/116",
		[]byte("v=0\r\n" +
			"o=- 1 1 IN IP4 127.0.0.1 \r\n" +
			"s=RTP session\r\n" +
			"e=NONE\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 MP4V-ES/1000\r\n" +
			"a=fmtp:96 profile-level-id=245; config=000001B0F5000001B509000001000000012000845D4C28582120A31F\r\n" +
			"a=framerate:25\r\n" +
			"a=x-dimensions:352,288\r\n" +
			"a=x-algoTarget:P\r\n" +
			"a=control:video\r\n"),
		[]byte("v=0\r\n" +
			"o=- 1 1 IN IP4 127.0.0.1\r\n" +
			"s=RTP session\r\n" +
			"e=NONE\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 MP4V-ES/1000\r\n" +
			"a=fmtp:96 profile-level-id=245; config=000001B0F5000001B509000001000000012000845D4C28582120A31F\r\n" +
			"a=framerate:25\r\n" +
			"a=x-dimensions:352,288\r\n" +
			"a=x-algoTarget:P\r\n" +
			"a=control:video\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      1,
				SessionVersion: 1,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "127.0.0.1",
			},
			SessionName: "RTP session",
			TimeDescriptions: []psdp.TimeDescription{
				{Timing: psdp.Timing{StartTime: 0, StopTime: 0}},
			},
			EmailAddress: func() *psdp.EmailAddress {
				e := psdp.EmailAddress("NONE")
				return &e
			}(),
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 MP4V-ES/1000",
						},
						{
							Key:   "fmtp",
							Value: "96 profile-level-id=245; config=000001B0F5000001B509000001000000012000845D4C28582120A31F",
						},
						{
							Key:   "framerate",
							Value: "25",
						},
						{
							Key:   "x-dimensions",
							Value: "352,288",
						},
						{
							Key:   "x-algoTarget",
							Value: "P",
						},
						{
							Key:   "control",
							Value: "video",
						},
					},
				},
			},
		},
	},
	{
		"issue gortsplib/112 a",
		[]byte("v=0\r\n" +
			"o=jdoe 0XAC4EC96E 2890842807 IN IP4 10.47.16.5\r\n" +
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
				{Timing: psdp.Timing{StartTime: 3034423619, StopTime: 3042462419}},
			},
		},
	},
	{
		"issue gortsplib/112 b",
		[]byte("v=0\r\n" +
			"o=jdoe 103bdb6f 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"i=A Seminar on the session description protocol\r\n" +
			"t=3034423619 3042462419\r\n"),
		[]byte("v=0\r\n" +
			"o=jdoe 272358255 2890842807 IN IP4 10.47.16.5\r\n" +
			"s=SDP Seminar\r\n" +
			"i=A Seminar on the session description protocol\r\n" +
			"t=3034423619 3042462419\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "jdoe",
				SessionID:      272358255,
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
				{Timing: psdp.Timing{StartTime: 3034423619, StopTime: 3042462419}},
			},
		},
	},
	{
		"issue mediamtx/948",
		[]byte("v=0\r\n" +
			"o=- 1681692777 1681692777 IN IP4 127.0.0.1\r\n" +
			"s=Video Stream\r\n" +
			"c=IN IP4 127.0.0.1\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=TIAS:10000\r\n" +
			"a=maxprate:2.0000\r\n" +
			"a=control:trackid=1\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=mimetype:string;\"video/H264\"\r\n" +
			"a=framesize:96 384-832\r\n" +
			"a=Width:integer;384\r\n" +
			"a=Height:integer;832\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=64001f;sprop-parameter-sets=J2QAH6xWwYBp+kA=,KO48sA==\r\n"),
		[]byte("v=0\r\n" +
			"o=- 1681692777 1681692777 IN IP4 127.0.0.1\r\n" +
			"s=Video Stream\r\n" +
			"c=IN IP4 127.0.0.1\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"b=TIAS:10000\r\n" +
			"a=maxprate:2.0000\r\n" +
			"a=control:trackid=1\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=mimetype:string;\"video/H264\"\r\n" +
			"a=framesize:96 384-832\r\n" +
			"a=Width:integer;384\r\n" +
			"a=Height:integer;832\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=64001f;sprop-parameter-sets=J2QAH6xWwYBp+kA=,KO48sA==\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      1681692777,
				SessionVersion: 1681692777,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "127.0.0.1",
			},
			SessionName: "Video Stream",
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address: &psdp.Address{
					Address: "127.0.0.1",
				},
			},
			TimeDescriptions: []psdp.TimeDescription{
				{},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "control",
					Value: "*",
				},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media: "video",
						Port: psdp.RangedPort{
							Value: 0,
						},
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Bandwidth: []psdp.Bandwidth{
						{
							Type:      "TIAS",
							Bandwidth: 10000,
						},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "maxprate",
							Value: "2.0000",
						},
						{
							Key:   "control",
							Value: "trackid=1",
						},
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key:   "mimetype",
							Value: "string;\"video/H264\"",
						},
						{
							Key:   "framesize",
							Value: "96 384-832",
						},
						{
							Key:   "Width",
							Value: "integer;384",
						},
						{
							Key:   "Height",
							Value: "integer;832",
						},
						{
							Key:   "fmtp",
							Value: "96 packetization-mode=1;profile-level-id=64001f;sprop-parameter-sets=J2QAH6xWwYBp+kA=,KO48sA==",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/990",
		[]byte("v=0\r\n" +
			"o=- 14665860 31787219 1 IN IP4 192.168.4.226\r\n" +
			"s=Session streamed by \"TP-LINK RTSP Server\"\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:4096\r\n" +
			"a=range:npt=0-\r\n" +
			"a=control:track1\r\n" +
			"a=rtpmap:96 H265/90000\r\n" +
			"a=fmtp:96 profile-space=0;profile-id=1;tier-flag=0;level-id=150;interop-constraints=000000000000;" +
			"sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ;" +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqABICAFEWNrkkya5ZwCAAADAAIAAAMAHhA=;" +
			"sprop-pps=RAHgdrAmQA==\r\n" +
			"m=audio 0 RTP/AVP 8\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n" +
			"a=control:track2\r\n" +
			"m=application/TP-LINK 0 RTP/AVP smart/1/90000\r\n" +
			"a=rtpmap:95 TP-LINK/90000\r\n" +
			"a=control:track3\r\n"),
		[]byte("v=0\r\n" +
			"o=- 14665860 31787219 1 IN IP4 192.168.4.226\r\n" +
			"s=Session streamed by \"TP-LINK RTSP Server\"\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:4096\r\n" +
			"a=range:npt=0-\r\n" +
			"a=control:track1\r\n" +
			"a=rtpmap:96 H265/90000\r\n" +
			"a=fmtp:96 profile-space=0;profile-id=1;tier-flag=0;level-id=150;interop-constraints=000000000000;" +
			"sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ;" +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqABICAFEWNrkkya5ZwCAAADAAIAAAMAHhA=;" +
			"sprop-pps=RAHgdrAmQA==\r\n" +
			"m=audio 0 RTP/AVP 8\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n" +
			"a=control:track2\r\n" +
			"m=application/TP-LINK 0 RTP/AVP smart/1/90000\r\n" +
			"a=rtpmap:95 TP-LINK/90000\r\n" +
			"a=control:track3\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "- 14665860",
				SessionID:      31787219,
				SessionVersion: 1,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "192.168.4.226",
			},
			SessionName:      "Session streamed by \"TP-LINK RTSP Server\"",
			TimeDescriptions: []psdp.TimeDescription{{}},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address: &psdp.Address{
							Address: "0.0.0.0",
						},
					},
					Bandwidth: []psdp.Bandwidth{{Type: "AS", Bandwidth: 4096}},
					Attributes: []psdp.Attribute{
						{
							Key:   "range",
							Value: "npt=0-",
						},
						{
							Key:   "control",
							Value: "track1",
						},
						{
							Key:   "rtpmap",
							Value: "96 H265/90000",
						},
						{
							Key: "fmtp",
							Value: "96 profile-space=0;profile-id=1;tier-flag=0;level-id=150;" +
								"interop-constraints=000000000000;sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ;" +
								"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqABICAFEWNrkkya5ZwCAAADAAIAAAMAHhA=;sprop-pps=RAHgdrAmQA==",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"8"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "8 PCMA/8000",
						},
						{
							Key:   "control",
							Value: "track2",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "application/TP-LINK",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"smart/1/90000"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "95 TP-LINK/90000",
						},
						{
							Key:   "control",
							Value: "track3",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/1119",
		[]byte("v=0\n" +
			"o=- 224 1 IN IP4 192.168.178.1\n" +
			"s=SatIPServer:1 0,0,4\n" +
			"t=0 0\n" +
			"m=video 0 RTP/AVP 33\n" +
			"c=In IP4 0.0.0.0\n" +
			"a=control:stream=1\n" +
			"a=fmtp:33 ver=1.2;src=1;tuner=1,240,1,7,112,,dvbc,,,,6900,34;pids=0,16,17,18,20\n" +
			"a=sendonly\n"),
		[]byte("v=0\r\n" +
			"o=- 224 1 IN IP4 192.168.178.1\r\n" +
			"s=SatIPServer:1 0,0,4\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 33\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=control:stream=1\r\n" +
			"a=fmtp:33 ver=1.2;src=1;tuner=1,240,1,7,112,,dvbc,,,,6900,34;pids=0,16,17,18,20\r\n" +
			"a=sendonly\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      224,
				SessionVersion: 1,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "192.168.178.1",
			},
			SessionName:      "SatIPServer:1 0,0,4",
			TimeDescriptions: []psdp.TimeDescription{{}},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"33"},
					},
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address: &psdp.Address{
							Address: "0.0.0.0",
						},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "control",
							Value: "stream=1",
						},
						{
							Key:   "fmtp",
							Value: "33 ver=1.2;src=1;tuner=1,240,1,7,112,,dvbc,,,,6900,34;pids=0,16,17,18,20",
						},
						{
							Key: "sendonly",
						},
					},
				},
			},
		},
	},
	{
		"onvif specification",
		[]byte("v=0\r\n" +
			"o= 2890842807 IN IP4 192.168.0.1\r\n" +
			"s=RTSP Session with audiobackchannel\r\n" +
			"m=video 0 RTP/AVP 26\r\n" +
			"a=control:rtsp://192.168.0.1/video\r\n" +
			"a=recvonly\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:rtsp://192.168.0.1/audio\r\n" +
			"a=recvonly\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:rtsp://192.168.0.1/audioback\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=sendonly\r\n"),
		[]byte("v=0\r\n" +
			"o= 0 2890842807 IN IP4 192.168.0.1\r\n" +
			"s=RTSP Session with audiobackchannel\r\n" +
			"m=video 0 RTP/AVP 26\r\n" +
			"a=control:rtsp://192.168.0.1/video\r\n" +
			"a=recvonly\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:rtsp://192.168.0.1/audio\r\n" +
			"a=recvonly\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:rtsp://192.168.0.1/audioback\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=sendonly\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				SessionVersion: 2890842807,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "192.168.0.1",
			},
			SessionName: "RTSP Session with audiobackchannel",
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"26"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "control",
							Value: "rtsp://192.168.0.1/video",
						},
						{
							Key: "recvonly",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"0"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "control",
							Value: "rtsp://192.168.0.1/audio",
						},
						{
							Key: "recvonly",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"0"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "control",
							Value: "rtsp://192.168.0.1/audioback",
						},
						{
							Key:   "rtpmap",
							Value: "0 PCMU/8000",
						},
						{
							Key: "sendonly",
						},
					},
				},
			},
		},
	},
	{
		"issue gortsplib/201",
		[]byte("v=0\n" +
			"o=JefferyZhang Inno Fuzhou 0 0 IN IP4 127.0.0.1\n" +
			"s=RbsLive\n" +
			"c=IN IP4 0.0.0.0\n" +
			"t=0 0\n" +
			"a=tool:libmpp at 2.0.1\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 profile-level-id=64C028;sprop-parameter-sets=Z2TAKKwa0A8ARPywDwiEag==,aO48sA==\n" +
			"a=control:track1\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 MPEG4-GENERIC/48000/2\n" +
			"a=fmtp:97 profile-level-id=15;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1190\n" +
			"a=control:track2\n"),
		[]byte("v=0\r\n" +
			"o=JefferyZhang Inno Fuzhou 0 0 IN IP4 127.0.0.1\r\n" +
			"s=RbsLive\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=tool:libmpp at 2.0.1\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 profile-level-id=64C028;sprop-parameter-sets=Z2TAKKwa0A8ARPywDwiEag==,aO48sA==\r\n" +
			"a=control:track1\r\n" +
			"m=audio 0 RTP/AVP 97\r\n" +
			"a=rtpmap:97 MPEG4-GENERIC/48000/2\r\n" +
			"a=fmtp:97 profile-level-id=15;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1190\r\n" +
			"a=control:track2\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "JefferyZhang Inno Fuzhou",
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "127.0.0.1",
			},
			SessionName: "RbsLive",
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address: &psdp.Address{
					Address: "0.0.0.0",
				},
			},
			TimeDescriptions: []psdp.TimeDescription{{}},
			Attributes: []psdp.Attribute{
				{
					Key:   "tool",
					Value: "libmpp at 2.0.1",
				},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
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
							Value: "96 profile-level-id=64C028;sprop-parameter-sets=Z2TAKKwa0A8ARPywDwiEag==,aO48sA==",
						},
						{
							Key:   "control",
							Value: "track1",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"97"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "97 MPEG4-GENERIC/48000/2",
						},
						{
							Key:   "fmtp",
							Value: "97 profile-level-id=15;mode=AAC-hbr;sizelength=13;indexlength=3;indexdeltalength=3;config=1190",
						},
						{
							Key:   "control",
							Value: "track2",
						},
					},
				},
			},
		},
	},
	{
		"issue gortsplib/271",
		[]byte("v=0\n" +
			"o=- 14665860 31787219 1 IN IP4 192.168.0.60\n" +
			"s=Session streamed by \"MERCURY RTSP Server\"\n" +
			"t=0 0\n" +
			"a=smart_encoder:virtualIFrame=1\n" +
			"m=video 0 RTP/AVP 96\n" +
			"c=IN IP4 0.0.0.0\n" +
			"b=AS:4096\n" +
			"a=range:npt=0-\n" +
			"a=control:track1\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=4D001F;" +
			" sprop-parameter-sets=J00AH+dAKALdgKUFBQXwAAADABAAAAMCi2gD6AXf//wK,KO48gA==\n" +
			"m=audio 0 RTP/AVP 8\n" +
			"a=rtpmap:8 PCMA/8000\n" +
			"a=control:track2\n" +
			"m=application/MERCURY 0 RTP/AVP smart/1/90000\n" +
			"a=rtpmap:95 MERCURY/90000\n" +
			"a=control:track3\n"),
		[]byte("v=0\r\n" +
			"o=- 14665860 31787219 1 IN IP4 192.168.0.60\r\n" +
			"s=Session streamed by \"MERCURY RTSP Server\"\r\n" +
			"t=0 0\r\n" +
			"a=smart_encoder:virtualIFrame=1\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:4096\r\n" +
			"a=range:npt=0-\r\n" +
			"a=control:track1\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=4D001F;" +
			" sprop-parameter-sets=J00AH+dAKALdgKUFBQXwAAADABAAAAMCi2gD6AXf//wK,KO48gA==\r\n" +
			"m=audio 0 RTP/AVP 8\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n" +
			"a=control:track2\r\n" +
			"m=application/MERCURY 0 RTP/AVP smart/1/90000\r\n" +
			"a=rtpmap:95 MERCURY/90000\r\n" +
			"a=control:track3\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "- 14665860",
				SessionID:      31787219,
				SessionVersion: 1,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "192.168.0.60",
			},
			SessionName:      "Session streamed by \"MERCURY RTSP Server\"",
			TimeDescriptions: []psdp.TimeDescription{{}},
			Attributes: []psdp.Attribute{
				{
					Key:   "smart_encoder",
					Value: "virtualIFrame=1",
				},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address: &psdp.Address{
							Address: "0.0.0.0",
						},
					},
					Bandwidth: []psdp.Bandwidth{{
						Type:      "AS",
						Bandwidth: 4096,
					}},
					Attributes: []psdp.Attribute{
						{
							Key:   "range",
							Value: "npt=0-",
						},
						{
							Key:   "control",
							Value: "track1",
						},
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key: "fmtp",
							Value: "96 packetization-mode=1; profile-level-id=4D001F;" +
								" sprop-parameter-sets=J00AH+dAKALdgKUFBQXwAAADABAAAAMCi2gD6AXf//wK,KO48gA==",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"8"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "8 PCMA/8000",
						},
						{
							Key:   "control",
							Value: "track2",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "application/MERCURY",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"smart/1/90000"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "95 MERCURY/90000",
						},
						{
							Key:   "control",
							Value: "track3",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/2128",
		[]byte("v=0\r\n" +
			"o=- 1 1 IN IPV4 10.10.10.10\r\n" +
			"s=Media Presentation\r\n" +
			"c=IN IPV4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"a=range:npt=now-\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=control:rtsp://10.10.10.10:5556/vurix/1414/0/video\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=64001F;" +
			"sprop-parameter-sets=Z2QAKKwbGoB4AiflwFuAgICgAAB9AAATiB0MAEr4AAL68F3lxoYAJXwAAX14LvLhQA==,aO48MA==\r\n" +
			"a=recvonly\r\n"),
		[]byte("v=0\r\n" +
			"o=- 1 1 IN IP4 10.10.10.10\r\n" +
			"s=Media Presentation\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"a=control:*\r\n" +
			"a=range:npt=now-\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=control:rtsp://10.10.10.10:5556/vurix/1414/0/video\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=64001F;" +
			"sprop-parameter-sets=Z2QAKKwbGoB4AiflwFuAgICgAAB9AAATiB0MAEr4AAL68F3lxoYAJXwAAX14LvLhQA==,aO48MA==\r\n" +
			"a=recvonly\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      1,
				SessionVersion: 1,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "10.10.10.10",
			},
			SessionName:      "Media Presentation",
			TimeDescriptions: []psdp.TimeDescription{{}},
			ConnectionInformation: &psdp.ConnectionInformation{
				NetworkType: "IN",
				AddressType: "IP4",
				Address: &psdp.Address{
					Address: "0.0.0.0",
				},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "control",
					Value: "*",
				},
				{
					Key:   "range",
					Value: "npt=now-",
				},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "control",
							Value: "rtsp://10.10.10.10:5556/vurix/1414/0/video",
						},
						{
							Key:   "rtpmap",
							Value: "96 H264/90000",
						},
						{
							Key: "fmtp",
							Value: "96 packetization-mode=1;profile-level-id=64001F;" +
								"sprop-parameter-sets=Z2QAKKwbGoB4AiflwFuAgICgAAB9AAATiB0MAEr4AAL68F3lxoYAJXwAAX14LvLhQA==,aO48MA==",
						},
						{
							Key:   "recvonly",
							Value: "",
						},
					},
				},
			},
		},
	},
	{
		"issue mediamtx/2473",
		[]byte("v=0\r\n" +
			"o=- 38990265062388 38990265062388 IN IP4 192.168.1.10\r\n" +
			"a=range:npt=0-\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtpmap:96 H265/90000 \r\n" +
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqABICAFEWNrkk5TNwEBAQQAAEZQAAV+QoQ=; sprop-pps=RAHA8vAiQA==\r\n" +
			"a=control:trackID=3\r\n" +
			"m=audio 0 RTP/AVP 8\r\n" +
			"a=control:trackID=4\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n"),
		[]byte("v=0\r\n" +
			"o=- 38990265062388 38990265062388 IN IP4 192.168.1.10\r\n" +
			"a=range:npt=0-\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtpmap:96 H265/90000 \r\n" +
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqABICAFEWNrkk5TNwEBAQQAAEZQAAV+QoQ=; sprop-pps=RAHA8vAiQA==\r\n" +
			"a=control:trackID=3\r\n" +
			"m=audio 0 RTP/AVP 8\r\n" +
			"a=control:trackID=4\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      38990265062388,
				SessionVersion: 38990265062388,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "192.168.1.10",
			},
			SessionName: "",
			Attributes: []psdp.Attribute{
				{
					Key:   "range",
					Value: "npt=0-",
				},
			},
			MediaDescriptions: []*psdp.MediaDescription{
				{
					MediaName: psdp.MediaName{
						Media:   "video",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"96"},
					},
					ConnectionInformation: &psdp.ConnectionInformation{
						NetworkType: "IN",
						AddressType: "IP4",
						Address: &psdp.Address{
							Address: "0.0.0.0",
						},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "rtpmap",
							Value: "96 H265/90000 ",
						},
						{
							Key: "fmtp",
							Value: "96 sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
								"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqABICAFEWNrkk5TNwEBAQQAAEZQAAV+QoQ=; sprop-pps=RAHA8vAiQA==",
						},
						{
							Key:   "control",
							Value: "trackID=3",
						},
					},
				},
				{
					MediaName: psdp.MediaName{
						Media:   "audio",
						Protos:  []string{"RTP", "AVP"},
						Formats: []string{"8"},
					},
					Attributes: []psdp.Attribute{
						{
							Key:   "control",
							Value: "trackID=4",
						},
						{
							Key:   "rtpmap",
							Value: "8 PCMA/8000",
						},
					},
				},
			},
		},
	},
	{
		"issue gortsplib/448",
		[]byte("m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=control:trackID=0\r\n"),
		[]byte("v=0\r\n" +
			"o= 0 0   \r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=control:trackID=0\r\n"),
		SessionDescription{
			MediaDescriptions: []*psdp.MediaDescription{{
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
						Key:   "control",
						Value: "trackID=0",
					},
				},
			}},
		},
	},
	{
		"issue mediamtx/2558",
		[]byte("v=0\r\n" +
			"o=- 1698210484.879535 1698210484.879535 IN IP4 46.242.10.231:12626\r\n" +
			"s=Playout\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=33;" +
			" sprop-parameter-sets=Z00AM4qKUDwBE/L/4AAgAC2AgA==,aO48gA==\r\n"),
		[]byte("v=0\r\n" +
			"o=- 97041581188 97041581188 IN IP4 46.242.10.231:12626\r\n" +
			"s=Playout\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=33;" +
			" sprop-parameter-sets=Z00AM4qKUDwBE/L/4AAgAC2AgA==,aO48gA==\r\n"),
		SessionDescription{
			Origin: psdp.Origin{
				Username:       "-",
				SessionID:      97041581188,
				SessionVersion: 97041581188,
				NetworkType:    "IN",
				AddressType:    "IP4",
				UnicastAddress: "46.242.10.231:12626",
			},
			SessionName: "Playout",
			MediaDescriptions: []*psdp.MediaDescription{{
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
						Value: "96 packetization-mode=1; profile-level-id=33; sprop-parameter-sets=Z00AM4qKUDwBE/L/4AAgAC2AgA==,aO48gA==",
					},
				},
			}},
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

func FuzzUnmarshal(f *testing.F) {
	f.Add("v=0\r\n" +
		"t=2873397496 2873404696\r\n" +
		"t=3034423619 3042462419\r\n" +
		"r=aa bb 0 90000\r\n")

	f.Add("v=0\r\n" +
		"t=2873397496 2873404696\r\n" +
		"t=3034423619 3042462419\r\n" +
		"r=123 bb 0 90000\r\n")

	f.Add("v=0\r\n" +
		"m=audio 49170 RTP/AVP 80000\r\n" +
		"i=Vivamus a posuere nisl\r\n" +
		"c=IN IP4 203.0.113.1\r\n" +
		"b=X-YZ:128\r\n" +
		"k=prompt\r\n" +
		"a=sendrecv\r\n")

	f.Add("v=0\r\n" +
		"o = IN \r\n")

	f.Fuzz(func(t *testing.T, b string) {
		desc := SessionDescription{}
		desc.Unmarshal([]byte(b)) //nolint:errcheck
	})
}
