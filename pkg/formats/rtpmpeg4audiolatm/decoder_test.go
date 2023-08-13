package rtpmpeg4audiolatm

import (
	"testing"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				Config: ca.config,
			}
			err := d.Init()
			require.NoError(t, err)

			var au []byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				au, _, err = d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
			}

			require.Equal(t, ca.au, au)
		})
	}
}

func TestDecodeOtherData(t *testing.T) {
	d := &Decoder{
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
			OtherDataPresent: true,
			OtherDataLenBits: 16,
		},
	}
	err := d.Init()
	require.NoError(t, err)

	au, _, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			Timestamp:      2289526357,
			SSRC:           2646308882,
		},
		Payload: []byte{
			0x04, 0x01, 0x02, 0x03, 0x04, 5, 6,
		},
	})
	require.NoError(t, err)

	require.Equal(t, []byte{1, 2, 3, 4}, au)
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{
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
		d.Init() //nolint:errcheck

		d.Decode(&rtp.Packet{ //nolint:errcheck
			Header: rtp.Header{
				Version:        2,
				Marker:         am,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: a,
		})

		d.Decode(&rtp.Packet{ //nolint:errcheck
			Header: rtp.Header{
				Version:        2,
				Marker:         bm,
				PayloadType:    96,
				SequenceNumber: 17646,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
