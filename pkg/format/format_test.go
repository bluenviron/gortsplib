package format

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
)

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

var casesFormat = []struct {
	name        string
	mediaType   string
	payloadType uint8
	rtpMap      string
	fmtp        map[string]string
	dec         Format
	encRtpMap   string
	encFmtp     map[string]string
}{
	{
		"audio g711 pcma",
		"audio",
		8,
		"",
		nil,
		&G711{},
		"PCMA/8000",
		nil,
	},
	{
		"audio g711 pcmu",
		"audio",
		0,
		"",
		nil,
		&G711{
			MULaw: true,
		},
		"PCMU/8000",
		nil,
	},
	{
		"audio g722",
		"audio",
		9,
		"",
		nil,
		&G722{},
		"G722/8000",
		nil,
	},
	{
		"audio g726 le 1",
		"audio",
		97,
		"G726-16/8000",
		nil,
		&G726{
			PayloadTyp: 97,
			BitRate:    16,
		},
		"G726-16/8000",
		nil,
	},
	{
		"audio g726 le 2",
		"audio",
		97,
		"G726-24/8000",
		nil,
		&G726{
			PayloadTyp: 97,
			BitRate:    24,
		},
		"G726-24/8000",
		nil,
	},
	{
		"audio g726 le 3",
		"audio",
		97,
		"G726-32/8000",
		nil,
		&G726{
			PayloadTyp: 97,
			BitRate:    32,
		},
		"G726-32/8000",
		nil,
	},
	{
		"audio g726 le 4",
		"audio",
		97,
		"G726-40/8000",
		nil,
		&G726{
			PayloadTyp: 97,
			BitRate:    40,
		},
		"G726-40/8000",
		nil,
	},
	{
		"audio g726 be",
		"audio",
		97,
		"AAL2-G726-32/8000",
		nil,
		&G726{
			PayloadTyp: 97,
			BitRate:    32,
			BigEndian:  true,
		},
		"AAL2-G726-32/8000",
		nil,
	},
	{
		"audio lpcm 8",
		"audio",
		97,
		"L8/48000/2",
		nil,
		&LPCM{
			PayloadTyp:   97,
			BitDepth:     8,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		"L8/48000/2",
		nil,
	},
	{
		"audio lpcm 16",
		"audio",
		97,
		"L16/96000/2",
		nil,
		&LPCM{
			PayloadTyp:   97,
			BitDepth:     16,
			SampleRate:   96000,
			ChannelCount: 2,
		},
		"L16/96000/2",
		nil,
	},
	{
		"audio lpcm 16 with no explicit channel",
		"audio",
		97,
		"L16/16000",
		nil,
		&LPCM{
			PayloadTyp:   97,
			BitDepth:     16,
			SampleRate:   16000,
			ChannelCount: 1,
		},
		"L16/16000/1",
		nil,
	},
	{
		"audio lpcm 24",
		"audio",
		98,
		"L24/44100/4",
		nil,
		&LPCM{
			PayloadTyp:   98,
			BitDepth:     24,
			SampleRate:   44100,
			ChannelCount: 4,
		},
		"L24/44100/4",
		nil,
	},
	{
		"audio mpeg2 audio",
		"audio",
		14,
		"",
		nil,
		&MPEG1Audio{},
		"",
		nil,
	},
	{
		"audio aac",
		"audio",
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"streamtype":       "5",
			"profile-level-id": "1",
			"mode":             "AAC-hbr",
			"sizelength":       "13",
			"indexlength":      "3",
			"indexdeltalength": "3",
			"config":           "11900810",
		},
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
		"audio",
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"profile-level-id": "1",
			"mode":             "AAC-hbr",
			"sizelength":       "13",
			"indexlength":      "3",
			"indexdeltalength": "3",
			"config":           "1190",
		},
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
		"audio",
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"streamtype":       "5",
			"profile-level-id": "14",
			"mode":             "AAC-hbr",
			"config":           "1190",
			"sizelength":       "13",
		},
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
		"audio",
		96,
		"mpeg4-generic/48000/2",
		map[string]string{
			"streamtype":       "5",
			"profile-level-id": "48",
			"mode":             "AAC-hbr",
			"config":           "eb098800",
			"sizelength":       "13",
		},
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
		"audio",
		96,
		"MP4A-LATM/24000/2",
		map[string]string{
			"profile-level-id": "1",
			"bitrate":          "64000",
			"cpresent":         "0",
			"object":           "2",
			"config":           "400026203fc0",
		},
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
		"audio",
		110,
		"MP4A-LATM/24000/1",
		map[string]string{
			"profile-level-id": "15",
			"object":           "2",
			"cpresent":         "0",
			"config":           "400026103fc0",
			"sbr-enabled":      "1",
		},
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
		"audio",
		110,
		"MP4A-LATM/48000/2",
		map[string]string{
			"profile-level-id": "44",
			"bitrate":          "64000",
			"cpresent":         "0",
			"config":           "40005623101fe0",
			"sbr-enabled":      "1",
		},
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
		"audio",
		110,
		"MP4A-LATM/48000/2",
		map[string]string{
			"profile-level-id": "48",
			"bitrate":          "64000",
			"cpresent":         "0",
			"config":           "4001d613101fe0",
		},
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
		"audio",
		110,
		"MP4A-LATM/48000",
		map[string]string{
			"cpresent": "0",
			"config":   "40002310",
		},
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
		"MP4A-LATM/48000/1",
		map[string]string{
			"profile-level-id": "30",
			"object":           "2",
			"cpresent":         "0",
			"config":           "400023103fc0",
		},
	},
	{
		"audio speex",
		"audio",
		96,
		"speex/16000",
		map[string]string{
			"vbr": "off",
		},
		&Speex{
			PayloadTyp: 96,
			SampleRate: 16000,
			VBR:        boolPtr(false),
		},
		"speex/16000",
		map[string]string{
			"vbr": "off",
		},
	},
	{
		"audio vorbis",
		"audio",
		96,
		"VORBIS/44100/2",
		map[string]string{
			"configuration": "AQIDBA==",
		},
		&Vorbis{
			PayloadTyp:    96,
			SampleRate:    44100,
			ChannelCount:  2,
			Configuration: []byte{0x01, 0x02, 0x03, 0x04},
		},
		"VORBIS/44100/2",
		map[string]string{
			"configuration": "AQIDBA==",
		},
	},
	{
		"audio opus",
		"audio",
		96,
		"opus/48000/2",
		map[string]string{
			"sprop-stereo": "1",
		},
		&Opus{
			PayloadTyp: 96,
			IsStereo:   true,
		},
		"opus/48000/2",
		map[string]string{
			"sprop-stereo": "1",
		},
	},
	{
		"audio ac3",
		"audio",
		96,
		"AC3/48000/2",
		nil,
		&AC3{
			PayloadTyp:   96,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		"AC3/48000/2",
		nil,
	},
	{
		"audio ac3 implicit channels",
		"audio",
		97,
		"AC3/48000",
		nil,
		&AC3{
			PayloadTyp:   97,
			SampleRate:   48000,
			ChannelCount: 6,
		},
		"AC3/48000/6",
		nil,
	},
	{
		"video jpeg",
		"video",
		26,
		"",
		nil,
		&MJPEG{},
		"JPEG/90000",
		nil,
	},
	{
		"video mpeg1 video",
		"video",
		32,
		"",
		nil,
		&MPEG1Video{},
		"",
		nil,
	},
	{
		"video mpeg-ts",
		"video",
		33,
		"",
		nil,
		&MPEGTS{},
		"MP2T/90000",
		nil,
	},
	{
		"video mpeg4 video",
		"video",
		96,
		"MP4V-ES/90000",
		map[string]string{
			"profile-level-id": "1",
			"config": "000001B001000001B58913000001000000012000C48" +
				"D8AEE053C04641443000001B24C61766335382E3133342E313030",
		},
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
		"MP4V-ES/90000",
		map[string]string{
			"profile-level-id": "1",
			"config": "000001B001000001B58913000001000000012000C48" +
				"D8AEE053C04641443000001B24C61766335382E3133342E313030",
		},
	},
	{
		"video h264",
		"video",
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"sprop-parameter-sets": "Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==",
			"profile-level-id":     "64000C",
		},
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
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"sprop-parameter-sets": "Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==",
			"profile-level-id":     "64000C",
		},
	},
	{
		"video h264 vlc rtsp server",
		"video",
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"profile-level-id":     "64001f",
			"sprop-parameter-sets": "Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,aOvjyyLA",
		},
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
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"profile-level-id":     "64001F",
			"sprop-parameter-sets": "Z2QAH6zZQFAFuwFsgAAAAwCAAAAeB4wYyw==,aOvjyyLA",
		},
	},
	{
		"video h264 sprop-parameter-sets with extra data",
		"video",
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"sprop-parameter-sets": "Z2QAKawTMUB4BEfeA+oCAgPgAAADACAAAAZSgA==,aPqPLA==,aF6jzAMA",
			"profile-level-id":     "640029",
		},
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
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"sprop-parameter-sets": "Z2QAKawTMUB4BEfeA+oCAgPgAAADACAAAAZSgA==,aPqPLA==",
			"profile-level-id":     "640029",
		},
	},
	{
		"video h264 empty sprop-parameter-sets",
		"video",
		96,
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"sprop-parameter-sets": "",
		},
		&H264{
			PayloadTyp:        96,
			PacketizationMode: 1,
		},
		"H264/90000",
		map[string]string{
			"packetization-mode": "1",
		},
	},
	{
		"video h264 annexb",
		"video",
		96,
		"H264/90000",
		map[string]string{
			"sprop-parameter-sets": "AAAAAWdNAB6NjUBaHtCAAAOEAACvyAI=,AAAAAWjuOIA=",
			"packetization-mode":   "1",
			"profile-level-id":     "4DE028",
		},
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
		"H264/90000",
		map[string]string{
			"packetization-mode":   "1",
			"profile-level-id":     "4D001E",
			"sprop-parameter-sets": "Z00AHo2NQFoe0IAAA4QAAK/IAg==,aO44gA==",
		},
	},
	{
		"video h264 with unparsable parameters (mediamtx/2348)",
		"video",
		96,
		"H264/90000",
		map[string]string{
			"sprop-parameter-sets": "QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Y2uSTJrlnAIAAADAAgAAAMAyEA=,RAHgdrAmQA==",
			"packetization-mode":   "1",
			"profile-level-id":     "010101",
		},
		&H264{
			PayloadTyp:        96,
			PacketizationMode: 1,
		},
		"H264/90000",
		map[string]string{
			"packetization-mode": "1",
		},
	},
	{
		"video h265",
		"video",
		96,
		"H265/90000",
		map[string]string{
			"sprop-vps":          "QAEMAf//AWAAAAMAkAAAAwAAAwB4mZgJ",
			"sprop-sps":          "QgEBAWAAAAMAkAAAAwAAAwB4oAPAgBDllmZpJMrgEAAAAwAQAAADAeCA",
			"sprop-pps":          "RAHBcrRiQA==",
			"sprop-max-don-diff": "2",
		},
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
		"video",
		96,
		"H265/90000",
		map[string]string{
			"sprop-vps": "AAAAAUABDAH//wFgAAADAAADAAADAAADAJasCQ==",
			"sprop-sps": "AAAAAUIBAQFgAAADAAADAAADAAADAJagBaIB4WNrkkya5Zk=",
			"sprop-pps": "AAAAAUQB4HawJkA=",
		},
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
		"H265/90000",
		map[string]string{
			"sprop-vps": "QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ",
			"sprop-sps": "QgEBAWAAAAMAAAMAAAMAAAMAlqAFogHhY2uSTJrlmQ==",
			"sprop-pps": "RAHgdrAmQA==",
		},
	},
	{
		"video vp8",
		"video",
		96,
		"VP8/90000",
		map[string]string{
			"max-fr": "123",
			"max-fs": "456",
		},
		&VP8{
			PayloadTyp: 96,
			MaxFR:      intPtr(123),
			MaxFS:      intPtr(456),
		},
		"VP8/90000",
		map[string]string{
			"max-fr": "123",
			"max-fs": "456",
		},
	},
	{
		"video vp9",
		"video",
		96,
		"VP9/90000",
		map[string]string{
			"max-fr":     "123",
			"max-fs":     "456",
			"profile-id": "789",
		},
		&VP9{
			PayloadTyp: 96,
			MaxFR:      intPtr(123),
			MaxFS:      intPtr(456),
			ProfileID:  intPtr(789),
		},
		"VP9/90000",
		map[string]string{
			"max-fr":     "123",
			"max-fs":     "456",
			"profile-id": "789",
		},
	},
	{
		"video av1",
		"video",
		96,
		"AV1/90000",
		map[string]string{
			"profile":   "2",
			"level-idx": "8",
			"tier":      "1",
		},
		&AV1{
			PayloadTyp: 96,
			Profile:    intPtr(2),
			LevelIdx:   intPtr(8),
			Tier:       intPtr(1),
		},
		"AV1/90000",
		map[string]string{
			"profile":   "2",
			"level-idx": "8",
			"tier":      "1",
		},
	},
	{
		"application",
		"application",
		98,
		"MetaData/80000",
		nil,
		&Generic{
			PayloadTyp: 98,
			RTPMa:      "MetaData/80000",
			ClockRat:   80000,
		},
		"MetaData/80000",
		nil,
	},
	{
		"application without clock rate",
		"application",
		107,
		"",
		nil,
		&Generic{
			PayloadTyp: 107,
		},
		"",
		nil,
	},
	{
		"application invalid rtpmap",
		"application",
		98,
		"custom",
		nil,
		&Generic{
			PayloadTyp: 98,
			RTPMa:      "custom",
		},
		"custom",
		nil,
	},
}

func TestUnmarshal(t *testing.T) {
	for _, ca := range casesFormat {
		t.Run(ca.name, func(t *testing.T) {
			dec, err := Unmarshal(ca.mediaType, ca.payloadType, ca.rtpMap, ca.fmtp)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}

func TestMarshal(t *testing.T) {
	for _, ca := range casesFormat {
		t.Run(ca.name, func(t *testing.T) {
			require.Equal(t, ca.payloadType, ca.dec.PayloadType())
			require.Equal(t, ca.encRtpMap, ca.dec.RTPMap())
			require.Equal(t, ca.encFmtp, ca.dec.FMTP())
		})
	}
}

func TestUnmarshalErrors(t *testing.T) {
	t.Run("invalid video", func(t *testing.T) {
		_, err := Unmarshal("video", 96, "", map[string]string{})
		require.Error(t, err)
	})

	t.Run("mpeg-4 audio generic", func(t *testing.T) {
		_, err := Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"streamtype": "10",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"mode": "asd",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"profile-level-id": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"config": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"config": "0ab2",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"sizelength": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"indexlength": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"indexdeltalength": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"profile-level-id": "1",
			"sizelength":       "13",
			"indexlength":      "3",
			"indexdeltalength": "3",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MPEG4-generic/48000/2", map[string]string{
			"profile-level-id": "1",
			"config":           "1190",
			"indexlength":      "3",
			"indexdeltalength": "3",
		})
		require.Error(t, err)
	})

	t.Run("mpeg-4 audio latm", func(t *testing.T) {
		_, err := Unmarshal("audio", 96, "MP4A-LATM/48000/2", map[string]string{
			"profile-level-id": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MP4A-LATM/48000/2", map[string]string{
			"bitrate": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MP4A-LATM/48000/2", map[string]string{
			"cpresent": "0",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MP4A-LATM/48000/2", map[string]string{
			"config": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("audio", 96, "MP4A-LATM/48000/2", map[string]string{
			"profile-level-id": "15",
			"object":           "2",
			"cpresent":         "0",
			"sbr-enabled":      "1",
		})
		require.Error(t, err)
	})

	t.Run("av1", func(t *testing.T) {
		_, err := Unmarshal("video", 96, "AV1/90000", map[string]string{
			"level-idx": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("video", 96, "AV1/90000", map[string]string{
			"profile": "aaa",
		})
		require.Error(t, err)

		_, err = Unmarshal("video", 96, "AV1/90000", map[string]string{
			"tier": "aaa",
		})
		require.Error(t, err)
	})
}

func FuzzUnmarshalH264(f *testing.F) {
	f.Fuzz(func(t *testing.T, sps string, pktMode string) {
		Unmarshal("video", 96, "H264/90000", map[string]string{ //nolint:errcheck
			"sprop-parameter-sets": sps,
			"packetization-mode":   pktMode,
		})
	})
}

func FuzzUnmarshalH265(f *testing.F) {
	f.Fuzz(func(t *testing.T, a, b, c, d string) {
		Unmarshal("video", 96, "H265/90000", map[string]string{ //nolint:errcheck
			"sprop-vps":          a,
			"sprop-sps":          b,
			"sprop-pps":          c,
			"sprop-max-don-diff": d,
		})
	})
}

func FuzzUnmarshalLPCM(f *testing.F) {
	f.Fuzz(func(t *testing.T, a string) {
		Unmarshal("audio", 96, "L16/"+a, nil) //nolint:errcheck
	})
}

func FuzzUnmarshalMPEG4Video(f *testing.F) {
	f.Fuzz(func(t *testing.T, a, b string) {
		Unmarshal("video", 96, "MP4V-ES/90000", map[string]string{ //nolint:errcheck
			"profile-level-id": a,
			"config":           b,
		})
	})
}

func FuzzUnmarshalOpus(f *testing.F) {
	f.Add("48000/a")

	f.Fuzz(func(t *testing.T, a string) {
		Unmarshal("audio", 96, "Opus/"+a, nil) //nolint:errcheck
	})
}

func FuzzUnmarshalVorbis(f *testing.F) {
	f.Fuzz(func(t *testing.T, a, b string) {
		Unmarshal("audio", 96, "Vorbis/"+a, map[string]string{ //nolint:errcheck
			"configuration": b,
		})
	})
}

func FuzzUnmarshalVP8(f *testing.F) {
	f.Fuzz(func(t *testing.T, a, b string) {
		Unmarshal("video", 96, "VP8/90000", map[string]string{ //nolint:errcheck
			"max-fr": a,
			"max-fs": b,
		})
	})
}

func FuzzUnmarshalVP9(f *testing.F) {
	f.Fuzz(func(t *testing.T, a, b, c string) {
		Unmarshal("video", 96, "VP9/90000", map[string]string{ //nolint:errcheck
			"max-fr":     a,
			"max-fs":     b,
			"profile-id": c,
		})
	})
}
