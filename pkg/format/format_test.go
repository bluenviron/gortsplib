package format

import (
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

var casesFormat = []struct {
	name           string
	in             string
	dec            Format
	encPayloadType uint8
	encRtpMap      string
	encFmtp        map[string]string
}{
	{
		"audio g711 pcma static payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 8\n",
		&G711{
			PayloadTyp:   8,
			MULaw:        false,
			SampleRate:   8000,
			ChannelCount: 1,
		},
		8,
		"PCMA/8000",
		nil,
	},
	{
		"audio g711 pcmu static payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 0\n",
		&G711{
			PayloadTyp:   0,
			MULaw:        true,
			SampleRate:   8000,
			ChannelCount: 1,
		},
		0,
		"PCMU/8000",
		nil,
	},
	{
		"audio g711 pcma dynamic payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 PCMA/16000/2",
		&G711{
			PayloadTyp:   96,
			MULaw:        false,
			SampleRate:   16000,
			ChannelCount: 2,
		},
		96,
		"PCMA/16000/2",
		nil,
	},
	{
		"audio g711 pcmu dynamic payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 PCMU/16000/2\n",
		&G711{
			PayloadTyp:   96,
			MULaw:        true,
			SampleRate:   16000,
			ChannelCount: 2,
		},
		96,
		"PCMU/16000/2",
		nil,
	},
	{
		"audio g722",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 9\n",
		&G722{},
		9,
		"G722/8000",
		nil,
	},
	{
		"audio g726 le 1",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 G726-16/8000\n",
		&G726{
			PayloadTyp: 97,
			BitRate:    16,
		},
		97,
		"G726-16/8000",
		nil,
	},
	{
		"audio g726 le 2",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 G726-24/8000\n",
		&G726{
			PayloadTyp: 97,
			BitRate:    24,
		},
		97,
		"G726-24/8000",
		nil,
	},
	{
		"audio g726 le 3",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 G726-32/8000\n",
		&G726{
			PayloadTyp: 97,
			BitRate:    32,
		},
		97,
		"G726-32/8000",
		nil,
	},
	{
		"audio g726 le 4",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 G726-40/8000\n",
		&G726{
			PayloadTyp: 97,
			BitRate:    40,
		},
		97,
		"G726-40/8000",
		nil,
	},
	{
		"audio g726 be",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 AAL2-G726-32/8000\n",
		&G726{
			PayloadTyp: 97,
			BitRate:    32,
			BigEndian:  true,
		},
		97,
		"AAL2-G726-32/8000",
		nil,
	},
	{
		"audio lpcm 8 dynamic payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 L8/48000/2\n",
		&LPCM{
			PayloadTyp:   97,
			BitDepth:     8,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		97,
		"L8/48000/2",
		nil,
	},
	{
		"audio lpcm 16 dynamic payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 L16/96000/2\n",
		&LPCM{
			PayloadTyp:   97,
			BitDepth:     16,
			SampleRate:   96000,
			ChannelCount: 2,
		},
		97,
		"L16/96000/2",
		nil,
	},
	{
		"audio lpcm 16 static payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 10\n",
		&LPCM{
			PayloadTyp:   10,
			BitDepth:     16,
			SampleRate:   44100,
			ChannelCount: 2,
		},
		10,
		"L16/44100/2",
		nil,
	},
	{
		"audio lpcm 16 static payload type",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 11\n",
		&LPCM{
			PayloadTyp:   11,
			BitDepth:     16,
			SampleRate:   44100,
			ChannelCount: 1,
		},
		11,
		"L16/44100/1",
		nil,
	},
	{
		"audio lpcm 16 with no explicit channel",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 L16/16000\n",
		&LPCM{
			PayloadTyp:   97,
			BitDepth:     16,
			SampleRate:   16000,
			ChannelCount: 1,
		},
		97,
		"L16/16000/1",
		nil,
	},
	{
		"audio lpcm 24",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 98\n" +
			"a=rtpmap:98 L24/44100/4\n",
		&LPCM{
			PayloadTyp:   98,
			BitDepth:     24,
			SampleRate:   44100,
			ChannelCount: 4,
		},
		98,
		"L24/44100/4",
		nil,
	},
	{
		"audio mpeg2 audio",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 14\n",
		&MPEG1Audio{},
		14,
		"",
		nil,
	},
	{
		"audio aac",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 mpeg4-generic/48000/2\n" +
			"a=fmtp:96 streamtype=5; profile-level-id=1; mode=AAC-hbr; " +
			"config=11900810; SizeLength=13; IndexLength=3; IndexDeltaLength=3\n",
		&MPEG4Audio{
			PayloadTyp:     96,
			ProfileLevelID: 1,
			Config: &mpeg4audio.Config{
				Type:         mpeg4audio.ObjectTypeAACLC,
				SampleRate:   48000,
				ChannelCount: 2,
			},
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		},
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"streamtype":       "5",
			"profile-level-id": "1",
			"mode":             "AAC-hbr",
			"sizelength":       "13",
			"indexlength":      "3",
			"indexdeltalength": "3",
			"config":           "1190",
		},
	},
	{
		"audio aac vlc rtsp server",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 mpeg4-generic/48000/2\n" +
			"a=fmtp:96 profile-level-id=1; mode=AAC-hbr; " +
			"config=1190; SizeLength=13; IndexLength=3; IndexDeltaLength=3\n",
		&MPEG4Audio{
			PayloadTyp:     96,
			ProfileLevelID: 1,
			Config: &mpeg4audio.Config{
				Type:         mpeg4audio.ObjectTypeAACLC,
				SampleRate:   48000,
				ChannelCount: 2,
			},
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		},
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"streamtype":       "5",
			"profile-level-id": "1",
			"mode":             "AAC-hbr",
			"sizelength":       "13",
			"indexlength":      "3",
			"indexdeltalength": "3",
			"config":           "1190",
		},
	},
	{
		"audio aac without indexlength",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 mpeg4-generic/48000/2\n" +
			"a=fmtp:96 streamtype=5; profile-level-id=14; mode=AAC-hbr; " +
			"config=1190; SizeLength=13\n",
		&MPEG4Audio{
			PayloadTyp:     96,
			ProfileLevelID: 14,
			Config: &mpeg4audio.Config{
				Type:         mpeg4audio.ObjectTypeAACLC,
				SampleRate:   48000,
				ChannelCount: 2,
			},
			SizeLength: 13,
		},
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"streamtype":       "5",
			"profile-level-id": "14",
			"mode":             "AAC-hbr",
			"config":           "1190",
			"sizelength":       "13",
		},
	},
	{
		"audio aac he-aac v2 ps",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 mpeg4-generic/48000/2\n" +
			"a=fmtp:96 streamtype=5; profile-level-id=48; mode=AAC-hbr; " +
			"config=eb098800; SizeLength=13\n",
		&MPEG4Audio{
			PayloadTyp:     96,
			ProfileLevelID: 48,
			Config: &mpeg4audio.Config{
				Type:                2,
				ExtensionType:       29,
				ExtensionSampleRate: 48000,
				SampleRate:          24000,
				ChannelCount:        1,
			},
			SizeLength: 13,
		},
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"streamtype":       "5",
			"profile-level-id": "48",
			"mode":             "AAC-hbr",
			"config":           "eb098800",
			"sizelength":       "13",
		},
	},
	{
		"audio aac latm lc",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 MP4A-LATM/24000/2\n" +
			"a=fmtp:96 profile-level-id=1; " +
			"bitrate=64000; cpresent=0; object=2; config=400026203fc0\n",
		&MPEG4Audio{
			LATM:           true,
			PayloadTyp:     96,
			ProfileLevelID: 1,
			Bitrate:        intPtr(64000),
			CPresent:       false,
			StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
				Programs: []*mpeg4audio.StreamMuxConfigProgram{{
					Layers: []*mpeg4audio.StreamMuxConfigLayer{{
						AudioSpecificConfig: &mpeg4audio.Config{
							Type:         2,
							SampleRate:   24000,
							ChannelCount: 2,
						},
						LatmBufferFullness: 255,
					}},
				}},
			},
		},
		96,
		"MP4A-LATM/24000/2",
		map[string]string{
			"profile-level-id": "1",
			"bitrate":          "64000",
			"cpresent":         "0",
			"object":           "2",
			"config":           "400026203fc0",
		},
	},
	{
		"audio aac latm he-aac v2",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 110\n" +
			"a=rtpmap:110 MP4A-LATM/24000/1\n" +
			"a=fmtp:110 profile-level-id=15; " +
			"cpresent=0; object=2; config=400026103fc0; sbr-enabled=1\n",
		&MPEG4Audio{
			LATM:           true,
			PayloadTyp:     110,
			ProfileLevelID: 15,
			CPresent:       false,
			SBREnabled:     boolPtr(true),
			StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
				Programs: []*mpeg4audio.StreamMuxConfigProgram{{
					Layers: []*mpeg4audio.StreamMuxConfigLayer{{
						AudioSpecificConfig: &mpeg4audio.Config{
							Type:         2,
							SampleRate:   24000,
							ChannelCount: 1,
						},
						LatmBufferFullness: 255,
					}},
				}},
			},
		},
		110,
		"MP4A-LATM/24000/1",
		map[string]string{
			"profile-level-id": "15",
			"object":           "2",
			"cpresent":         "0",
			"config":           "400026103fc0",
			"SBR-enabled":      "1",
		},
	},
	{
		"audio aac latm hierarchical sbr",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 110\n" +
			"a=rtpmap:110 MP4A-LATM/48000/2\n" +
			"a=fmtp:110 profile-level-id=44; " +
			"bitrate=64000; cpresent=0; config=40005623101fe0; sbr-enabled=1\n",
		&MPEG4Audio{
			LATM:           true,
			PayloadTyp:     110,
			ProfileLevelID: 44,
			CPresent:       false,
			SBREnabled:     boolPtr(true),
			Bitrate:        intPtr(64000),
			StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
				Programs: []*mpeg4audio.StreamMuxConfigProgram{{
					Layers: []*mpeg4audio.StreamMuxConfigLayer{{
						AudioSpecificConfig: &mpeg4audio.Config{
							Type:                2,
							ExtensionType:       5,
							ExtensionSampleRate: 48000,
							SampleRate:          24000,
							ChannelCount:        2,
						},
						LatmBufferFullness: 255,
					}},
				}},
			},
		},
		110,
		"MP4A-LATM/48000/2",
		map[string]string{
			"profile-level-id": "44",
			"object":           "2",
			"cpresent":         "0",
			"config":           "40005623101fe0",
			"SBR-enabled":      "1",
			"bitrate":          "64000",
		},
	},
	{
		"audio aac latm hierarchical ps",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 110\n" +
			"a=rtpmap:110 MP4A-LATM/48000/2\n" +
			"a=fmtp:110 profile-level-id=48; " +
			"bitrate=64000; cpresent=0; config=4001d613101fe0\n",
		&MPEG4Audio{
			LATM:           true,
			PayloadTyp:     110,
			ProfileLevelID: 48,
			Bitrate:        intPtr(64000),
			CPresent:       false,
			StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
				Programs: []*mpeg4audio.StreamMuxConfigProgram{{
					Layers: []*mpeg4audio.StreamMuxConfigLayer{{
						AudioSpecificConfig: &mpeg4audio.Config{
							Type:                2,
							ExtensionType:       29,
							ExtensionSampleRate: 48000,
							SampleRate:          24000,
							ChannelCount:        1,
						},
						LatmBufferFullness: 255,
					}},
				}},
			},
		},
		110,
		"MP4A-LATM/48000/2",
		map[string]string{
			"profile-level-id": "48",
			"object":           "2",
			"cpresent":         "0",
			"config":           "4001d613101fe0",
			"bitrate":          "64000",
		},
	},
	{
		"audio aac latm no channels",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 110\n" +
			"a=rtpmap:110 MP4A-LATM/48000\n" +
			"a=fmtp:110 profile-level-id=30; " +
			"cpresent=0; config=40002310\n",
		&MPEG4Audio{
			LATM:           true,
			PayloadTyp:     110,
			ProfileLevelID: 30,
			CPresent:       false,
			StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
				Programs: []*mpeg4audio.StreamMuxConfigProgram{{
					Layers: []*mpeg4audio.StreamMuxConfigLayer{{
						AudioSpecificConfig: &mpeg4audio.Config{
							Type:         2,
							SampleRate:   48000,
							ChannelCount: 1,
						},
						LatmBufferFullness: 255,
					}},
				}},
			},
		},
		110,
		"MP4A-LATM/48000/1",
		map[string]string{
			"profile-level-id": "30",
			"object":           "2",
			"cpresent":         "0",
			"config":           "400023103fc0",
		},
	},
	{
		"audio aac latm cpresent",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 MP4A-LATM/48000/2\n" +
			"a=fmtp:96 cpresent=1\n",
		&MPEG4Audio{
			PayloadTyp:     96,
			LATM:           true,
			ProfileLevelID: 30,
			CPresent:       true,
		},
		96,
		"MP4A-LATM/16000/1",
		map[string]string{
			"cpresent":         "1",
			"profile-level-id": "30",
		},
	},
	{
		"audio speex",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 speex/16000\n" +
			"a=fmtp:96 vbr=off\n",
		&Speex{
			PayloadTyp: 96,
			SampleRate: 16000,
			VBR:        boolPtr(false),
		},
		96,
		"speex/16000",
		map[string]string{
			"vbr": "off",
		},
	},
	{
		"audio vorbis",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 VORBIS/44100/2\n" +
			"a=fmtp:96 configuration=AQIDBA==\n",
		&Vorbis{
			PayloadTyp:    96,
			SampleRate:    44100,
			ChannelCount:  2,
			Configuration: []byte{0x01, 0x02, 0x03, 0x04},
		},
		96,
		"VORBIS/44100/2",
		map[string]string{
			"configuration": "AQIDBA==",
		},
	},
	{
		"audio opus",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 opus/48000/2\n" +
			"a=fmtp:96 sprop-stereo=1\n",
		&Opus{
			PayloadTyp:   96,
			IsStereo:     true,
			ChannelCount: 2,
		},
		96,
		"opus/48000/2",
		map[string]string{
			"sprop-stereo": "1",
		},
	},
	{
		"audio opus 5.1",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 multiopus/48000/6\n" +
			"a=fmtp:96 num_streams=4; coupled_streams=2; channel_mapping=0,4,1,2,3,5\n",
		&Opus{
			PayloadTyp:   96,
			ChannelCount: 6,
		},
		96,
		"multiopus/48000/6",
		map[string]string{
			"channel_mapping":      "0,4,1,2,3,5",
			"coupled_streams":      "2",
			"num_streams":          "4",
			"sprop-maxcapturerate": "48000",
		},
	},
	{
		"audio ac3",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 96\n" +
			"a=rtpmap:96 AC3/48000/2\n",
		&AC3{
			PayloadTyp:   96,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		96,
		"AC3/48000/2",
		nil,
	},
	{
		"audio ac3 implicit channels",
		"v=0\n" +
			"s=\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=rtpmap:97 AC3/48000\n",
		&AC3{
			PayloadTyp:   97,
			SampleRate:   48000,
			ChannelCount: 6,
		},
		97,
		"AC3/48000/6",
		nil,
	},
	{
		"video jpeg",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 26\n",
		&MJPEG{},
		26,
		"JPEG/90000",
		nil,
	},
	{
		"video mpeg1 video",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 32\n",
		&MPEG1Video{},
		32,
		"",
		nil,
	},
	{
		"video mpeg-ts",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 33\n",
		&MPEGTS{},
		33,
		"MP2T/90000",
		nil,
	},
	{
		"video mpeg4 video",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 MP4V-ES/90000\n" +
			"a=fmtp:96 profile-level-id=1; " +
			"config=000001B001000001B58913000001000000012000C48D8AEE053C04641443000001B24C61766335382E3133342E313030\n",
		&MPEG4Video{
			PayloadTyp:     96,
			ProfileLevelID: 1,
			Config: []byte{
				0x00, 0x00, 0x01, 0xb0, 0x01, 0x00, 0x00, 0x01,
				0xb5, 0x89, 0x13, 0x00, 0x00, 0x01, 0x00, 0x00,
				0x00, 0x01, 0x20, 0x00, 0xc4, 0x8d, 0x8a, 0xee,
				0x05, 0x3c, 0x04, 0x64, 0x14, 0x43, 0x00, 0x00,
				0x01, 0xb2, 0x4c, 0x61, 0x76, 0x63, 0x35, 0x38,
				0x2e, 0x31, 0x33, 0x34, 0x2e, 0x31, 0x30, 0x30,
			},
		},
		96,
		"MP4V-ES/90000",
		map[string]string{
			"profile-level-id": "1",
			"config": "000001B001000001B58913000001000000012000C48" +
				"D8AEE053C04641443000001B24C61766335382E3133342E313030",
		},
	},
	{
		"video h264",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=64000C; " +
			"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==\n",
		&H264{
			PayloadTyp: 96,
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
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"sprop-parameter-sets": "Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==",
			"profile-level-id":     "64000C",
		},
	},
	{
		"video h264 vlc rtsp server",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=64001f; " +
			"sprop-parameter-sets=Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,aOvjyyLA\n",
		&H264{
			PayloadTyp: 96,
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
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"profile-level-id":     "64001F",
			"sprop-parameter-sets": "Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,aOvjyyLA",
		},
	},
	{
		"video h264 sprop-parameter-sets with extra data",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=640029; " +
			"sprop-parameter-sets=Z2QAKawTMUB4BEfeA+oCAgPgAAADACAAAAZSgA==,aPqPLA==,aF6jzAMA\n",
		&H264{
			PayloadTyp: 96,
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
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"sprop-parameter-sets": "Z2QAKawTMUB4BEfeA+oCAgPgAAADACAAAAZSgA==,aPqPLA==",
			"profile-level-id":     "640029",
		},
	},
	{
		"video h264 empty sprop-parameter-sets",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 packetization-mode=1; sprop-parameter-sets=\n",
		&H264{
			PayloadTyp:        96,
			PacketizationMode: 1,
		},
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode": "1",
		},
	},
	{
		"video h264 annexb",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=4DE028; " +
			"sprop-parameter-sets=AAAAAWdNAB6NjUBaHtCAAAOEAACvyAI=,AAAAAWjuOIA=\n",
		&H264{
			PayloadTyp: 96,
			SPS: []byte{
				0x67, 0x4d, 0x00, 0x1e, 0x8d, 0x8d, 0x40, 0x5a,
				0x1e, 0xd0, 0x80, 0x00, 0x03, 0x84, 0x00, 0x00,
				0xaf, 0xc8, 0x02,
			},
			PPS: []byte{
				0x68, 0xee, 0x38, 0x80,
			},
			PacketizationMode: 1,
		},
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"profile-level-id":     "4D001E",
			"sprop-parameter-sets": "Z00AHo2NQFoe0IAAA4QAAK/IAg==,aO44gA==",
		},
	},
	{
		"video h264 with unparsable parameters (mediamtx/2348)",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H264/90000\n" +
			"a=fmtp:96 packetization-mode=1; profile-level-id=010101; " +
			"sprop-parameter-sets=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Y2uSTJrlnAIAAADAAgAAAMAyEA=,RAHgdrAmQA==\n",
		&H264{
			PayloadTyp:        96,
			PacketizationMode: 1,
		},
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode": "1",
		},
	},
	{
		"video h264 with space at end",
		"v=0\r\n" +
			"o=- 4158123474391860926 2 IN IP4 127.0.0.1\r\n" +
			"s=-\r\n" +
			"t=0 0\r\n" +
			"m=video 42504 RTP/AVP 96\r\n" +
			"a=rtpmap:96 H264/90000 \r\n" +
			"a=fmtp:96 packetization-mode=1\r\n",
		&H264{
			PayloadTyp:        96,
			PacketizationMode: 1,
		},
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode": "1",
		},
	},
	{
		"video h264 bosch (issue gortsplib/632)",
		"v=0\n" +
			"o=- 0 0 IN IP4 10.100.14.102\n" +
			"s=LIVE VIEW\n" +
			"c=IN IP4 0.0.0.0\n" +
			"t=0 0\n" +
			"a=control:rtsp://10.100.14.102:554/?inst=2&h26x=4\n" +
			"m=video 0 RTP/AVP 35\n" +
			"a=rtpmap:35 H264/90000\n" +
			"a=control:rtsp://10.100.14.102:554/?inst=2&h26x=4&stream=video\n" +
			"a=recvonly\n" +
			"a=fmtp:35 packetization-mode=1;profile-level-id=4d4029;sprop-parameter-sets=Z01AKY2NYDwBE/LgLcBDQECA,aO44gA==\n",
		&H264{
			PayloadTyp: 35,
			SPS: []byte{
				0x67, 0x4d, 0x40, 0x29, 0x8d, 0x8d, 0x60, 0x3c,
				0x01, 0x13, 0xf2, 0xe0, 0x2d, 0xc0, 0x43, 0x40,
				0x40, 0x80,
			},
			PPS: []byte{
				0x68, 0xee, 0x38, 0x80,
			},
			PacketizationMode: 1,
		},
		35,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"profile-level-id":     "4D4029",
			"sprop-parameter-sets": "Z01AKY2NYDwBE/LgLcBDQECA,aO44gA==",
		},
	},
	{
		"video h265",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H265/90000\n" +
			"a=fmtp:96 sprop-vps=QAEMAf//AWAAAAMAkAAAAwAAAwB4mZgJ; " +
			"sprop-sps=QgEBAWAAAAMAkAAAAwAAAwB4oAPAgBDllmZpJMrgEAAAAwAQAAADAeCA; " +
			"sprop-pps=RAHBcrRiQA==; sprop-max-don-diff=2\n",
		&H265{
			PayloadTyp: 96,
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
		96,
		"H265/90000",
		map[string]string{
			"sprop-vps":          "QAEMAf//AWAAAAMAkAAAAwAAAwB4mZgJ",
			"sprop-sps":          "QgEBAWAAAAMAkAAAAwAAAwB4oAPAgBDllmZpJMrgEAAAAwAQAAADAeCA",
			"sprop-pps":          "RAHBcrRiQA==",
			"sprop-max-don-diff": "2",
		},
	},
	{
		"video h265 annexb",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 H265/90000\n" +
			"a=fmtp:96 sprop-vps=AAAAAUABDAH//wFgAAADAAADAAADAAADAJasCQ==; " +
			"sprop-sps=AAAAAUIBAQFgAAADAAADAAADAAADAJagBaIB4WNrkkya5Zk=; " +
			"sprop-pps=AAAAAUQB4HawJkA=\n",
		&H265{
			PayloadTyp: 96,
			VPS: []byte{
				0x40, 0x01, 0x0c, 0x01, 0xff, 0xff, 0x01, 0x60,
				0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x03, 0x00, 0x00, 0x03, 0x00, 0x96, 0xac, 0x09,
			},
			SPS: []byte{
				0x42, 0x01, 0x01, 0x01, 0x60, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x03, 0x00, 0x96, 0xa0, 0x05, 0xa2, 0x01, 0xe1,
				0x63, 0x6b, 0x92, 0x4c, 0x9a, 0xe5, 0x99,
			},
			PPS: []byte{
				0x44, 0x01, 0xe0, 0x76, 0xb0, 0x26, 0x40,
			},
		},
		96,
		"H265/90000",
		map[string]string{
			"sprop-vps": "QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ",
			"sprop-sps": "QgEBAWAAAAMAAAMAAAMAAAMAlqAFogHhY2uSTJrlmQ==",
			"sprop-pps": "RAHgdrAmQA==",
		},
	},
	{
		"video vp8",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 VP8/90000\n" +
			"a=fmtp:96 max-fr=123; max-fs=456\n",
		&VP8{
			PayloadTyp: 96,
			MaxFR:      intPtr(123),
			MaxFS:      intPtr(456),
		},
		96,
		"VP8/90000",
		map[string]string{
			"max-fr": "123",
			"max-fs": "456",
		},
	},
	{
		"video vp9",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 VP9/90000\n" +
			"a=fmtp:96 max-fr=123; max-fs=456; profile-id=789\n",
		&VP9{
			PayloadTyp: 96,
			MaxFR:      intPtr(123),
			MaxFS:      intPtr(456),
			ProfileID:  intPtr(789),
		},
		96,
		"VP9/90000",
		map[string]string{
			"max-fr":     "123",
			"max-fs":     "456",
			"profile-id": "789",
		},
	},
	{
		"video av1",
		"v=0\n" +
			"s=\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=rtpmap:96 AV1/90000\n" +
			"a=fmtp:96 profile=2; level-idx=8; tier=1\n",
		&AV1{
			PayloadTyp: 96,
			Profile:    intPtr(2),
			LevelIdx:   intPtr(8),
			Tier:       intPtr(1),
		},
		96,
		"AV1/90000",
		map[string]string{
			"profile":   "2",
			"level-idx": "8",
			"tier":      "1",
		},
	},
	{
		"application",
		"v=0\n" +
			"s=\n" +
			"m=application 0 RTP/AVP 98\n" +
			"a=rtpmap:98 MetaData/80000\n",
		&Generic{
			PayloadTyp: 98,
			RTPMa:      "MetaData/80000",
			ClockRat:   80000,
		},
		98,
		"MetaData/80000",
		nil,
	},
	{
		"application without clock rate",
		"v=0\n" +
			"s=\n" +
			"m=application 0 RTP/AVP 107\n",
		&Generic{
			PayloadTyp: 107,
		},
		107,
		"",
		nil,
	},
	{
		"application invalid rtpmap",
		"v=0\n" +
			"s=\n" +
			"m=application 0 RTP/AVP 98\n" +
			"a=rtpmap:98 custom\n",
		&Generic{
			PayloadTyp: 98,
			RTPMa:      "custom",
		},
		98,
		"custom",
		nil,
	},
	{
		"application tp-link (issue gortsplib/509)",
		"v=0\n" +
			"o=- 14665860 31787219 1 IN IP4 192.168.1.102\n" +
			"s=Session streamed by \"TP-LINK RTSP Server\"\n" +
			"t=0 0\n" +
			"a=smart_encoder:virtualIFrame=1\n" +
			"m=application/tp-link 0 RTP/AVP smart/0/25000\n" +
			"a=rtpmap:95 tp-link/25000\n" +
			"a=control:track3\n",
		&Generic{
			PayloadTyp: 95,
			RTPMa:      "tp-link/25000",
			ClockRat:   25000,
		},
		95,
		"tp-link/25000",
		nil,
	},
	{
		"application mercury (issue gortsplib/271)",
		"v=0\n" +
			"o=- 14665860 31787219 1 IN IP4 192.168.0.60\n" +
			"s=Session streamed by \"MERCURY RTSP Server\"\n" +
			"t=0 0\n" +
			"a=smart_encoder:virtualIFrame=1\n" +
			"m=application/MERCURY 0 RTP/AVP smart/1/90000\n" +
			"a=rtpmap:95 MERCURY/90000\n",
		&Generic{
			PayloadTyp: 95,
			RTPMa:      "MERCURY/90000",
			ClockRat:   90000,
		},
		95,
		"MERCURY/90000",
		nil,
	},
	{
		"application tp-link (issue mediamtx/1267)",
		"v=0\r\n" +
			"o=- 4158123474391860926 2 IN IP4 127.0.0.1\r\n" +
			"s=-\r\n" +
			"t=0 0\r\n" +
			"m=application/TP-LINK 0 RTP/AVP smart/1/90000\r\n" +
			"a=rtpmap:95 TP-LINK/90000\r\n",
		&Generic{
			PayloadTyp: 95,
			RTPMa:      "TP-LINK/90000",
			ClockRat:   90000,
		},
		95,
		"TP-LINK/90000",
		nil,
	},
	{
		"audio aac from AVOIP (issue mediamtx/4183)",
		"v=0\r\n" +
			"o=- 1634883673031268 1 IN IP4 127.0.0.1\r\n" +
			"s=H265 Video, streamed by the LIVE555 Media Server\r\n" +
			"i=test.265\r\n" +
			"t=0 0\r\n" +
			"a=tool:LIVE555 Streaming Media v2016.10.11\r\n" +
			"a=type:broadcast\r\n" +
			"a=control:*\r\n" +
			"a=range:npt=0-\r\n" +
			"a=x-qt-text-nam:H.265 Video, streamed by the LIVE555 Media Server\r\n" +
			"a=x-qt-text-inf:test.265\r\n" +
			"m=audio 0 RTP/AVP 100\r\n" +
			"a=rtpmap:100 mpeg4-generic/48000/2\r\n" +
			"a=fmtp:100 streamtype=5; sizeLength=13; indexLength=3; indexDeltaLength=3; mode=AAC_hbr; config=1190\r\n" +
			"a=control:track1\r\n",
		&MPEG4Audio{
			PayloadTyp: 100,
			Config: &mpeg4audio.AudioSpecificConfig{
				Type:         2,
				SampleRate:   48000,
				ChannelCount: 2,
			},
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		},
		100,
		"mpeg4-generic/48000/2",
		map[string]string{
			"config":           "1190",
			"indexdeltalength": "3",
			"indexlength":      "3",
			"mode":             "AAC-hbr",
			"profile-level-id": "1",
			"sizelength":       "13",
			"streamtype":       "5",
		},
	},
}

func TestUnmarshal(t *testing.T) {
	for _, ca := range casesFormat {
		t.Run(ca.name, func(t *testing.T) {
			var desc sdp.SessionDescription
			err := desc.Unmarshal([]byte(ca.in))
			require.NoError(t, err)
			require.Equal(t, 1, len(desc.MediaDescriptions))
			require.Equal(t, 1, len(desc.MediaDescriptions[0].MediaName.Formats))

			dec, err := Unmarshal(desc.MediaDescriptions[0], desc.MediaDescriptions[0].MediaName.Formats[0])
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}

func TestMarshal(t *testing.T) {
	for _, ca := range casesFormat {
		t.Run(ca.name, func(t *testing.T) {
			require.Equal(t, ca.encPayloadType, ca.dec.PayloadType())
			require.Equal(t, ca.encRtpMap, ca.dec.RTPMap())
			require.Equal(t, ca.encFmtp, ca.dec.FMTP())
		})
	}
}

func FuzzUnmarshal(f *testing.F) {
	for _, ca := range casesFormat {
		f.Add(ca.in)
	}

	f.Fuzz(func(_ *testing.T, in string) {
		var desc sdp.SessionDescription
		err := desc.Unmarshal([]byte(in))

		if err == nil && len(desc.MediaDescriptions) == 1 && len(desc.MediaDescriptions[0].MediaName.Formats) == 1 {
			Unmarshal(desc.MediaDescriptions[0], desc.MediaDescriptions[0].MediaName.Formats[0]) //nolint:errcheck
		}
	})
}
