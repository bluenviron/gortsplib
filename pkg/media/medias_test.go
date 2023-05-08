package media

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v3/pkg/formats"
	"github.com/bluenviron/gortsplib/v3/pkg/sdp"
)

var casesMedias = []struct {
	name   string
	in     string
	out    string
	medias Medias
}{
	{
		"one format for each media, absolute",
		"v=0\r\n" +
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
			"b=AS:8\r\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 97\r\n" +
			"a=control:rtsp://10.0.100.50/profile5/media.smp/trackID=v\r\n" +
			"a=rtpmap:97 H264/90000\r\n" +
			"a=fmtp:97 packetization-mode=1; profile-level-id=640028; sprop-parameter-sets=Z2QAKKy0A8ARPyo=,aO4Bniw=\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:rtsp://10.0.100.50/profile5/media.smp/trackID=a\r\n" +
			"a=recvonly\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"m=application 0 RTP/AVP 107\r\n" +
			"a=control\r\n",
		Medias{
			{
				Type:    "video",
				Control: "rtsp://10.0.100.50/profile5/media.smp/trackID=v",
				Formats: []formats.Format{&formats.H264{
					PayloadTyp:        97,
					PacketizationMode: 1,
					SPS:               []byte{0x67, 0x64, 0x00, 0x28, 0xac, 0xb4, 0x03, 0xc0, 0x11, 0x3f, 0x2a},
					PPS:               []byte{0x68, 0xee, 0x01, 0x9e, 0x2c},
				}},
			},
			{
				Type:      "audio",
				Direction: DirectionRecvonly,
				Control:   "rtsp://10.0.100.50/profile5/media.smp/trackID=a",
				Formats: []formats.Format{&formats.G711{
					MULaw: true,
				}},
			},
			{
				Type: "application",
				Formats: []formats.Format{&formats.Generic{
					PayloadTyp: 107,
				}},
			},
		},
	},
	{
		"one format for each media, relative",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 10.0.0.131\r\n" +
			"s=Media Presentation\r\n" +
			"i=samsung\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"b=AS:2632\r\n" +
			"t=0 0\r\n" +
			"a=range:npt=now-\r\n" +
			"m=video 42504 RTP/AVP 97\r\n" +
			"b=AS:2560\r\n" +
			"a=rtpmap:97 H264/90000\r\n" +
			"a=control:trackID=1\r\n" +
			"a=cliprect:0,0,1080,1920\r\n" +
			"a=framesize:97 1920-1080\r\n" +
			"a=framerate:30.0\r\n" +
			"a=fmtp:97 packetization-mode=1;profile-level-id=640028;sprop-parameter-sets=Z2QAKKy0A8ARPyo=,aO4Bniw=\r\n" +
			"m=audio 42506 RTP/AVP 0\r\n" +
			"b=AS:64\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=control:trackID=2\r\n" +
			"a=recvonly\r\n" +
			"m=application 42508 RTP/AVP 107\r\n" +
			"b=AS:8\r\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 97\r\n" +
			"a=control:trackID=1\r\n" +
			"a=rtpmap:97 H264/90000\r\n" +
			"a=fmtp:97 packetization-mode=1; profile-level-id=640028; sprop-parameter-sets=Z2QAKKy0A8ARPyo=,aO4Bniw=\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:trackID=2\r\n" +
			"a=recvonly\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"m=application 0 RTP/AVP 107\r\n" +
			"a=control\r\n",
		Medias{
			{
				Type:    "video",
				Control: "trackID=1",
				Formats: []formats.Format{&formats.H264{
					PayloadTyp:        97,
					PacketizationMode: 1,
					SPS:               []byte{0x67, 0x64, 0x00, 0x28, 0xac, 0xb4, 0x03, 0xc0, 0x11, 0x3f, 0x2a},
					PPS:               []byte{0x68, 0xee, 0x01, 0x9e, 0x2c},
				}},
			},
			{
				Type:      "audio",
				Direction: DirectionRecvonly,
				Control:   "trackID=2",
				Formats: []formats.Format{&formats.G711{
					MULaw: true,
				}},
			},
			{
				Type: "application",
				Formats: []formats.Format{&formats.Generic{
					PayloadTyp: 107,
				}},
			},
		},
	},
	{
		"multiple formats for each media",
		"v=0\r\n" +
			"o=- 4158123474391860926 2 IN IP4 127.0.0.1\r\n" +
			"s=-\r\n" +
			"t=0 0\r\n" +
			"a=group:BUNDLE audio video\r\n" +
			"a=msid-semantic: WMS mediaStreamLocal\r\n" +
			"m=audio 9 UDP/TLS/RTP/SAVPF 111 103 104 9 102 0 8 106 105 13 110 112 113 126\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtcp:9 IN IP4 0.0.0.0\r\n" +
			"a=ice-ufrag:0D6Y\r\n" +
			"a=ice-pwd:V3YEqLGAJJhUDUa13C/pKbWe\r\n" +
			"a=ice-options:trickle renomination\r\n" +
			"a=fingerprint:sha-256" +
			" 5E:B5:97:8B:B4:D8:AE:2B:89:F6:82:44:47:69:77:83:05:29:C5:C8:EE:67:50:C3:77:6B:A7:BA:10:E3:08:B8\r\n" +
			"a=setup:actpass\r\n" +
			"a=mid:audio\r\n" +
			"a=extmap:1 urn:ietf:params:rtp-hdrext:ssrc-audio-level\r\n" +
			"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\n" +
			"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\r\n" +
			"a=sendonly\r\n" +
			"a=rtcp-mux\r\n" +
			"a=rtpmap:111 opus/48000/2\r\n" +
			"a=rtcp-fb:111 transport-cc\r\n" +
			"a=fmtp:111 minptime=10;useinbandfec=1\r\n" +
			"a=rtpmap:103 ISAC/16000\r\n" +
			"a=rtpmap:104 ISAC/32000\r\n" +
			"a=rtpmap:9 G722/8000\r\n" +
			"a=rtpmap:102 ILBC/8000\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n" +
			"a=rtpmap:106 CN/32000\r\n" +
			"a=rtpmap:105 CN/16000\r\n" +
			"a=rtpmap:13 CN/8000\r\n" +
			"a=rtpmap:110 telephone-event/48000\r\n" +
			"a=rtpmap:112 telephone-event/32000\r\n" +
			"a=rtpmap:113 telephone-event/16000\r\n" +
			"a=rtpmap:126 telephone-event/8000\r\n" +
			"a=ssrc:3754810229 cname:CvU1TYqkVsjj5XOt\r\n" +
			"a=ssrc:3754810229 msid:mediaStreamLocal 101\r\n" +
			"a=ssrc:3754810229 mslabel:mediaStreamLocal\r\n" +
			"a=ssrc:3754810229 label:101\r\n" +
			"m=video 9 UDP/TLS/RTP/SAVPF 96 97 98 99 100 101 127 124 125\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"a=rtcp:9 IN IP4 0.0.0.0\r\n" +
			"a=ice-ufrag:0D6Y\r\n" +
			"a=ice-pwd:V3YEqLGAJJhUDUa13C/pKbWe\r\n" +
			"a=ice-options:trickle renomination\r\n" +
			"a=fingerprint:sha-256" +
			" 5E:B5:97:8B:B4:D8:AE:2B:89:F6:82:44:47:69:77:83:05:29:C5:C8:EE:67:50:C3:77:6B:A7:BA:10:E3:08:B8\r\n" +
			"a=setup:actpass\r\n" +
			"a=mid:video\r\n" +
			"a=extmap:14 urn:ietf:params:rtp-hdrext:toffset\r\n" +
			"a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time\r\n" +
			"a=extmap:13 urn:3gpp:video-orientation\r\n" +
			"a=extmap:3 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01\r\n" +
			"a=extmap:5 http://www.webrtc.org/experiments/rtp-hdrext/playout-delay\r\n" +
			"a=extmap:6 http://www.webrtc.org/experiments/rtp-hdrext/video-content-type\r\n" +
			"a=extmap:7 http://www.webrtc.org/experiments/rtp-hdrext/video-timing\r\n" +
			"a=extmap:8 http://www.webrtc.org/experiments/rtp-hdrext/color-space\r\n" +
			"a=sendonly\r\n" +
			"a=rtcp-mux\r\n" +
			"a=rtcp-rsize\r\n" +
			"a=rtpmap:96 VP8/90000\r\n" +
			"a=rtcp-fb:96 goog-remb\r\n" +
			"a=rtcp-fb:96 transport-cc\r\n" +
			"a=rtcp-fb:96 ccm fir\r\n" +
			"a=rtcp-fb:96 nack\r\n" +
			"a=rtcp-fb:96 nack pli\r\n" +
			"a=rtpmap:97 rtx/90000\r\n" +
			"a=fmtp:97 apt=96\r\n" +
			"a=rtpmap:98 VP9/90000\r\n" +
			"a=rtcp-fb:98 goog-remb\r\n" +
			"a=rtcp-fb:98 transport-cc\r\n" +
			"a=rtcp-fb:98 ccm fir\r\n" +
			"a=rtcp-fb:98 nack\r\n" +
			"a=rtcp-fb:98 nack pli\r\n" +
			"a=rtpmap:99 rtx/90000\r\n" +
			"a=fmtp:99 apt=98\r\n" +
			"a=rtpmap:100 H264/90000\r\n" +
			"a=rtcp-fb:100 goog-remb\r\n" +
			"a=rtcp-fb:100 transport-cc\r\n" +
			"a=rtcp-fb:100 ccm fir\r\n" +
			"a=rtcp-fb:100 nack\r\n" +
			"a=rtcp-fb:100 nack pli\r\n" +
			"a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f\r\n" +
			"a=rtpmap:101 rtx/90000\r\n" +
			"a=fmtp:101 apt=100\r\n" +
			"a=rtpmap:127 red/90000\r\n" +
			"a=rtpmap:124 rtx/90000\r\n" +
			"a=fmtp:124 apt=127\r\n" +
			"a=rtpmap:125 ulpfec/90000\r\n" +
			"a=ssrc-group:FID 2712436124 1733091158\r\n" +
			"a=ssrc:2712436124 cname:CvU1TYqkVsjj5XOt\r\n" +
			"a=ssrc:2712436124 msid:mediaStreamLocal 100\r\n" +
			"a=ssrc:2712436124 mslabel:mediaStreamLocal\r\n" +
			"a=ssrc:2712436124 label:100\r\n" +
			"a=ssrc:1733091158 cname:CvU1TYqkVsjj5XOt\r\n" +
			"a=ssrc:1733091158 msid:mediaStreamLocal 100\r\n" +
			"a=ssrc:1733091158 mslabel:mediaStreamLocal\r\n" +
			"a=ssrc:1733091158 label:100\r\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=audio 0 RTP/AVP 111 103 104 9 102 0 8 106 105 13 110 112 113 126\r\n" +
			"a=control\r\n" +
			"a=sendonly\r\n" +
			"a=rtpmap:111 opus/48000/2\r\n" +
			"a=fmtp:111 sprop-stereo=0\r\n" +
			"a=rtpmap:103 ISAC/16000\r\n" +
			"a=rtpmap:104 ISAC/32000\r\n" +
			"a=rtpmap:9 G722/8000\r\n" +
			"a=rtpmap:102 ILBC/8000\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"a=rtpmap:8 PCMA/8000\r\n" +
			"a=rtpmap:106 CN/32000\r\n" +
			"a=rtpmap:105 CN/16000\r\n" +
			"a=rtpmap:13 CN/8000\r\n" +
			"a=rtpmap:110 telephone-event/48000\r\n" +
			"a=rtpmap:112 telephone-event/32000\r\n" +
			"a=rtpmap:113 telephone-event/16000\r\n" +
			"a=rtpmap:126 telephone-event/8000\r\n" +
			"m=video 0 RTP/AVP 96 97 98 99 100 101 127 124 125\r\n" +
			"a=control\r\n" +
			"a=sendonly\r\n" +
			"a=rtpmap:96 VP8/90000\r\n" +
			"a=rtpmap:97 rtx/90000\r\n" +
			"a=fmtp:97 apt=96\r\n" +
			"a=rtpmap:98 VP9/90000\r\n" +
			"a=rtpmap:99 rtx/90000\r\n" +
			"a=fmtp:99 apt=98\r\n" +
			"a=rtpmap:100 H264/90000\r\n" +
			"a=fmtp:100 packetization-mode=1\r\n" +
			"a=rtpmap:101 rtx/90000\r\n" +
			"a=fmtp:101 apt=100\r\n" +
			"a=rtpmap:127 red/90000\r\n" +
			"a=rtpmap:124 rtx/90000\r\n" +
			"a=fmtp:124 apt=127\r\na=rtpmap:125 ulpfec/90000\r\n",
		Medias{
			{
				Type:      "audio",
				Direction: DirectionSendonly,
				Formats: []formats.Format{
					&formats.Opus{
						PayloadTyp: 111,
						IsStereo:   false,
					},
					&formats.Generic{
						PayloadTyp: 103,
						RTPMa:      "ISAC/16000",
						ClockRat:   16000,
					},
					&formats.Generic{
						PayloadTyp: 104,
						RTPMa:      "ISAC/32000",
						ClockRat:   32000,
					},
					&formats.G722{},
					&formats.Generic{
						PayloadTyp: 102,
						RTPMa:      "ILBC/8000",
						ClockRat:   8000,
					},
					&formats.G711{
						MULaw: true,
					},
					&formats.G711{
						MULaw: false,
					},
					&formats.Generic{
						PayloadTyp: 106,
						RTPMa:      "CN/32000",
						ClockRat:   32000,
					},
					&formats.Generic{
						PayloadTyp: 105,
						RTPMa:      "CN/16000",
						ClockRat:   16000,
					},
					&formats.Generic{
						PayloadTyp: 13,
						RTPMa:      "CN/8000",
						ClockRat:   8000,
					},
					&formats.Generic{
						PayloadTyp: 110,
						RTPMa:      "telephone-event/48000",
						ClockRat:   48000,
					},
					&formats.Generic{
						PayloadTyp: 112,
						RTPMa:      "telephone-event/32000",
						ClockRat:   32000,
					},
					&formats.Generic{
						PayloadTyp: 113,
						RTPMa:      "telephone-event/16000",
						ClockRat:   16000,
					},
					&formats.Generic{
						PayloadTyp: 126,
						RTPMa:      "telephone-event/8000",
						ClockRat:   8000,
					},
				},
			},
			{
				Type:      "video",
				Direction: DirectionSendonly,
				Formats: []formats.Format{
					&formats.VP8{
						PayloadTyp: 96,
					},
					&formats.Generic{
						PayloadTyp: 97,
						RTPMa:      "rtx/90000",
						FMT: map[string]string{
							"apt": "96",
						},
						ClockRat: 90000,
					},
					&formats.VP9{
						PayloadTyp: 98,
					},
					&formats.Generic{
						PayloadTyp: 99,
						RTPMa:      "rtx/90000",
						FMT: map[string]string{
							"apt": "98",
						},
						ClockRat: 90000,
					},
					&formats.H264{
						PayloadTyp:        100,
						PacketizationMode: 1,
					},
					&formats.Generic{
						PayloadTyp: 101,
						RTPMa:      "rtx/90000",
						FMT: map[string]string{
							"apt": "100",
						},
						ClockRat: 90000,
					},
					&formats.Generic{
						PayloadTyp: 127,
						RTPMa:      "red/90000",
						ClockRat:   90000,
					},
					&formats.Generic{
						PayloadTyp: 124,
						RTPMa:      "rtx/90000",
						FMT: map[string]string{
							"apt": "127",
						},
						ClockRat: 90000,
					},
					&formats.Generic{
						PayloadTyp: 125,
						RTPMa:      "ulpfec/90000",
						ClockRat:   90000,
					},
				},
			},
		},
	},
	{
		"multiple formats for each media 2",
		"v=0\r\n" +
			"o=- 4158123474391860926 2 IN IP4 127.0.0.1\r\n" +
			"s=-\r\n" +
			"t=0 0\r\n" +
			"m=video 42504 RTP/AVP 96 98\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=rtpmap:98 MetaData\r\n" +
			"a=rtcp-mux\r\n" +
			"a=fmtp:96 packetization-mode=1;profile-level-id=4d002a;" +
			"sprop-parameter-sets=Z00AKp2oHgCJ+WbgICAgQA==,aO48gA==\r\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96 98\r\n" +
			"a=control\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=4D002A; " +
			"sprop-parameter-sets=Z00AKp2oHgCJ+WbgICAgQA==,aO48gA==\r\n" +
			"a=rtpmap:98 MetaData\r\n",
		Medias{
			{
				Type: "video",
				Formats: []formats.Format{
					&formats.H264{
						PayloadTyp: 96,
						SPS: []byte{
							0x67, 0x4d, 0x00, 0x2a, 0x9d, 0xa8, 0x1e, 0x00,
							0x89, 0xf9, 0x66, 0xe0, 0x20, 0x20, 0x20, 0x40,
						},
						PPS:               []byte{0x68, 0xee, 0x3c, 0x80},
						PacketizationMode: 1,
					},
					&formats.Generic{
						PayloadTyp: 98,
						RTPMa:      "MetaData",
					},
				},
			},
		},
	},
	{
		"onvif back channel",
		"v=0\r\n" +
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
			"a=sendonly\r\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 26\r\n" +
			"a=control:rtsp://192.168.0.1/video\r\n" +
			"a=recvonly\r\n" +
			"a=rtpmap:26 JPEG/90000\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:rtsp://192.168.0.1/audio\r\n" +
			"a=recvonly\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n" +
			"m=audio 0 RTP/AVP 0\r\n" +
			"a=control:rtsp://192.168.0.1/audioback\r\n" +
			"a=sendonly\r\n" +
			"a=rtpmap:0 PCMU/8000\r\n",
		Medias{
			{
				Type:      "video",
				Direction: DirectionRecvonly,
				Control:   "rtsp://192.168.0.1/video",
				Formats:   []formats.Format{&formats.MJPEG{}},
			},
			{
				Type:      "audio",
				Direction: DirectionRecvonly,
				Control:   "rtsp://192.168.0.1/audio",
				Formats:   []formats.Format{&formats.G711{MULaw: true}},
			},
			{
				Type:      "audio",
				Direction: DirectionSendonly,
				Control:   "rtsp://192.168.0.1/audioback",
				Formats:   []formats.Format{&formats.G711{MULaw: true}},
			},
		},
	},
	{
		"tp-link",
		"v=0\r\n" +
			"o=- 4158123474391860926 2 IN IP4 127.0.0.1\r\n" +
			"s=-\r\n" +
			"t=0 0\r\n" +
			"m=application/TP-LINK 0 RTP/AVP smart/1/90000\r\n" +
			"a=rtpmap:95 TP-LINK/90000\r\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=application/TP-LINK 0 RTP/AVP 95\r\n" +
			"a=control\r\n" +
			"a=rtpmap:95 TP-LINK/90000\r\n",
		Medias{
			{
				Type: "application/TP-LINK",
				Formats: []formats.Format{&formats.Generic{
					PayloadTyp: 95,
					RTPMa:      "TP-LINK/90000",
					ClockRat:   90000,
				}},
			},
		},
	},
	{
		"mercury",
		"v=0\n" +
			"o=- 14665860 31787219 1 IN IP4 192.168.0.60\n" +
			"s=Session streamed by \"MERCURY RTSP Server\"\n" +
			"t=0 0\n" +
			"a=smart_encoder:virtualIFrame=1\n" +
			"m=application/MERCURY 0 RTP/AVP smart/1/90000\n" +
			"a=rtpmap:95 MERCURY/90000\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=application/MERCURY 0 RTP/AVP 95\r\n" +
			"a=control\r\n" +
			"a=rtpmap:95 MERCURY/90000\r\n",
		Medias{
			{
				Type: "application/MERCURY",
				Formats: []formats.Format{&formats.Generic{
					PayloadTyp: 95,
					RTPMa:      "MERCURY/90000",
					ClockRat:   90000,
				}},
			},
		},
	},
	{
		"h264 with space at end",
		"v=0\r\n" +
			"o=- 4158123474391860926 2 IN IP4 127.0.0.1\r\n" +
			"s=-\r\n" +
			"t=0 0\r\n" +
			"m=video 42504 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000 \r\n" +
			"a=fmtp:96 packetization-mode=1\r\n",
		"v=0\r\n" +
			"o=- 0 0 IN IP4 127.0.0.1\r\n" +
			"s=Stream\r\n" +
			"c=IN IP4 0.0.0.0\r\n" +
			"t=0 0\r\n" +
			"m=video 0 RTP/AVP 96\r\n" +
			"a=control\r\n" +
			"a=rtpmap:96 H264/90000\r\n" +
			"a=fmtp:96 packetization-mode=1\r\n",
		Medias{
			{
				Type: "video",
				Formats: []formats.Format{
					&formats.H264{
						PayloadTyp:        96,
						PacketizationMode: 1,
					},
				},
			},
		},
	},
}

func TestMediasUnmarshal(t *testing.T) {
	for _, ca := range casesMedias {
		t.Run(ca.name, func(t *testing.T) {
			var sdp sdp.SessionDescription
			err := sdp.Unmarshal([]byte(ca.in))
			require.NoError(t, err)

			var medias Medias
			err = medias.Unmarshal(sdp.MediaDescriptions)
			require.NoError(t, err)
			require.Equal(t, ca.medias, medias)
		})
	}
}

func TestMediasUnmarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		sdp  string
		err  string
	}{
		{
			"invalid track",
			"v=0\r\n" +
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
				"a=control:streamid=1\r\n",
			"media 2 is invalid: invalid AAC config: zzz1210",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var sd sdp.SessionDescription
			err := sd.Unmarshal([]byte(ca.sdp))
			require.NoError(t, err)

			var medias Medias
			err = medias.Unmarshal(sd.MediaDescriptions)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestMediasMarshal(t *testing.T) {
	for _, ca := range casesMedias {
		t.Run(ca.name, func(t *testing.T) {
			byts, err := ca.medias.Marshal(false).Marshal()
			require.NoError(t, err)
			require.Equal(t, ca.out, string(byts))
		})
	}
}

func TestMediasFindFormat(t *testing.T) {
	tr := &formats.Generic{
		PayloadTyp: 97,
		RTPMa:      "rtx/90000",
		FMT: map[string]string{
			"apt": "96",
		},
		ClockRat: 90000,
	}

	md := &Media{
		Type: TypeVideo,
		Formats: []formats.Format{
			&formats.VP8{
				PayloadTyp: 96,
			},
			tr,
			&formats.VP9{
				PayloadTyp: 98,
			},
		},
	}

	ms := Medias{
		{
			Type: TypeAudio,
			Formats: []formats.Format{
				&formats.Opus{
					PayloadTyp: 111,
					IsStereo:   true,
				},
			},
		},
		md,
	}

	var forma *formats.Generic
	me := ms.FindFormat(&forma)
	require.Equal(t, md, me)
	require.Equal(t, tr, forma)
}
