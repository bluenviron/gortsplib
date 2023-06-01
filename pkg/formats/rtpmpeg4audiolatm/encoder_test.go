package rtpmpeg4audiolatm

import (
	"bytes"
	"testing"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func mergeBytes(vals ...[]byte) []byte {
	size := 0
	for _, v := range vals {
		size += len(v)
	}
	res := make([]byte, size)

	pos := 0
	for _, v := range vals {
		n := copy(res[pos:], v)
		pos += n
	}

	return res
}

var cases = []struct {
	name   string
	config *mpeg4audio.StreamMuxConfig
	au     []byte
	pkts   []*rtp.Packet
}{
	{
		"single",
		&mpeg4audio.StreamMuxConfig{
			Programs: []*mpeg4audio.StreamMuxConfigProgram{{
				Layers: []*mpeg4audio.StreamMuxConfigLayer{{
					AudioSpecificConfig: &mpeg4audio.AudioSpecificConfig{
						Type:         2,
						SampleRate:   48000,
						ChannelCount: 2,
					},
					LatmBufferFullness: 255,
				}},
			}},
		},
		[]byte{1, 2, 3, 4},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           2646308882,
				},
				Payload: []byte{
					0x04, 0x01, 0x02, 0x03, 0x04,
				},
			},
		},
	},
	{
		"fragmented",
		&mpeg4audio.StreamMuxConfig{
			Programs: []*mpeg4audio.StreamMuxConfigProgram{{
				Layers: []*mpeg4audio.StreamMuxConfigLayer{{
					AudioSpecificConfig: &mpeg4audio.AudioSpecificConfig{
						Type:         2,
						SampleRate:   48000,
						ChannelCount: 2,
					},
					LatmBufferFullness: 255,
				}},
			}},
		},
		bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 512),
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					bytes.Repeat([]byte{0xff}, 16),
					[]byte{0x10},
					bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 180),
					[]byte{0, 1, 2},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289526357,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					[]byte{3, 4, 5, 6, 7},
					bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 181),
					[]byte{0, 1, 2, 3, 4, 5, 6},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289526357,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					[]byte{7},
					bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 149),
				),
			},
		},
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				PayloadType:           96,
				Config:                ca.config,
				SSRC:                  uint32Ptr(0x9dbb7812),
				InitialSequenceNumber: uint16Ptr(0x44ed),
				InitialTimestamp:      uint32Ptr(0x88776655),
			}
			e.Init()

			pkts, err := e.Encode(ca.au, 0)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{
		PayloadType: 96,
		Config: &mpeg4audio.StreamMuxConfig{
			Programs: []*mpeg4audio.StreamMuxConfigProgram{{
				Layers: []*mpeg4audio.StreamMuxConfigLayer{{
					AudioSpecificConfig: &mpeg4audio.AudioSpecificConfig{
						Type:         2,
						SampleRate:   48000,
						ChannelCount: 2,
					},
					LatmBufferFullness: 255,
				}},
			}},
		},
	}
	e.Init()
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
	require.NotEqual(t, nil, e.InitialTimestamp)
}
