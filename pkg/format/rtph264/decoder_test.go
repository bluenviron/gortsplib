package rtph264

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var au [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				var addNALUs [][]byte
				addNALUs, err = d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if errors.Is(err, ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				au = append(au, addNALUs...)
			}

			require.Equal(t, ca.au, au)
		})
	}
}

var casesDecodeOnly = []struct {
	name string
	pkts []*rtp.Packet
	au   [][]byte
}{
	{
		"corrupted fragment",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289527317,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{
						0x1c, 0x85,
					},
					bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
					[]byte{0x00, 0x01},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289527317,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x01, 0x00},
			},
		},
		[][]byte{{0x01, 0x00}},
	},
	{
		"issue gortsplib/649 (CostarHD, FU-A with both start and end bit set)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 18853,
					Timestamp:      1731630255,
					SSRC:           0x466b0000,
				},
				Payload: mergeBytes(
					[]byte{
						0x3c,       // FU indicator
						0xc1,       // FU header (start and end bit both intentionally set)
						0xe7, 0x00, // DON
						0xca, 0xfe, // Payload
					},
				),
			},
		},
		[][]byte{{0x21, 0xe7, 0x00, 0xca, 0xfe}},
	},
	{
		"STAP-A with padding",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x18, 0x00, 0x02, 0xaa,
					0xbb, 0x00, 0x02, 0xcc, 0xdd, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
					0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
				},
			},
		},
		[][]byte{
			{0xaa, 0xbb},
			{0xcc, 0xdd},
		},
	},
	{
		"issue gortsplib/199 (AnnexB-encoded streams, single NALU)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x01, 0x02, 0x03, 0x04},
				),
			},
		},
		[][]byte{
			{0x01, 0x02, 0x03, 0x04},
		},
	},
	{
		"issue gortsplib/199 (AnnexB-encoded streams, multiple NALUs)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x00, 0x00, 0x00, 0x01},
					[]byte{0x01, 0x02, 0x03, 0x04},
					[]byte{0x00, 0x00, 0x00, 0x01},
					[]byte{0x01, 0x02, 0x03, 0x04},
				),
			},
		},
		[][]byte{
			{0x01, 0x02, 0x03, 0x04},
			{0x01, 0x02, 0x03, 0x04},
		},
	},
	{
		"marker-splitted access units",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{1, 2},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{3, 4},
			},
		},
		[][]byte{{1, 2}, {3, 4}},
	},
	{
		"issue mediamtx/3945 (FLIR M400, timestamp-splitted access units)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{1, 2},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531308,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{3, 4},
			},
		},
		[][]byte{{1, 2}},
	},
	{
		"issue gortsplib/989 (Amatek AR-N3222F, multiple NALUs in FU, basic)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 54972,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x7c, 0x87, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50,
					0x28, 0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa,
					0x00, 0x00, 0x30, 0xd4,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 54973,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x7c, 0x47, 0x01, 0x00, 0x00, 0x00, 0x01, 0x68,
					0xee, 0x3c, 0x80,
				},
			},
		},
		[][]byte{
			{
				0x67, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50, 0x28,
				0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa, 0x00,
				0x00, 0x30, 0xd4, 0x01,
			},
			{
				0x68, 0xee, 0x3c, 0x80,
			},
		},
	},
	{
		"issue gortsplib/989 (Amatek AR-N3222F, multiple NALUs in FU, leading start code)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 54972,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x80, 0x00, 0x01, 0x67, 0x4d, 0x00, 0x33,
					0x8a, 0x8a, 0x50, 0x28, 0x02, 0xdd, 0x34, 0x40,
					0x00, 0x00, 0xfa, 0x00,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 54973,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x40, 0x00, 0x30, 0xd4, 0x01, 0x00, 0x00,
					0x01, 0x68, 0xee, 0x3c, 0x80,
				},
			},
		},
		[][]byte{
			{
				0x67, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50, 0x28,
				0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa, 0x00,
				0x00, 0x30, 0xd4, 0x01,
			},
			{
				0x68, 0xee, 0x3c, 0x80,
			},
		},
	},
	{
		"issue gortsplib/989 (Amatek AR-N3222F, multiple NALUs in FU, leading and trailing start codes)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 54972,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x80, 0x00, 0x01, 0x67, 0x4d, 0x00, 0x33,
					0x8a, 0x8a, 0x50, 0x28, 0x02, 0xdd, 0x34, 0x40,
					0x00, 0x00, 0xfa, 0x00,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 54973,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x40, 0x00, 0x30, 0xd4, 0x01, 0x00, 0x00,
					0x01, 0x68, 0xee, 0x3c, 0x80, 0x00, 0x00, 0x00,
					0x01,
				},
			},
		},
		[][]byte{
			{
				0x67, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50, 0x28,
				0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa, 0x00,
				0x00, 0x30, 0xd4, 0x01,
			},
			{
				0x68, 0xee, 0x3c, 0x80,
			},
		},
	},
}

func TestDecodeOnly(t *testing.T) {
	for _, ca := range casesDecodeOnly {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var au [][]byte

			for i, pkt := range ca.pkts {
				au, err = d.Decode(pkt)

				if i != len(ca.pkts)-1 {
					require.ErrorIs(t, err, ErrMorePacketsNeeded)
				} else {
					require.NoError(t, err)
				}
			}

			require.Equal(t, ca.au, au)
		})
	}
}

func TestDecodeErrorNALUSize(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	size := 0
	i := uint16(0)

	for size < h264.MaxAccessUnitSize {
		flags := byte(0)
		if size == 0 {
			flags = 0b10000000
		}

		_, err = d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645 + i,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: append(
				[]byte{byte(h264.NALUTypeFUA), flags},
				bytes.Repeat([]byte{1, 2, 3, 4}, 1400/4)...,
			),
		})

		size += 1400
		i++
	}

	require.EqualError(t, err, "NALU size (8388801) is too big, maximum is 8388608")
}

func TestDecodeErrorNALUCount(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	for i := 0; i <= h264.MaxNALUsPerAccessUnit; i++ {
		_, err = d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{1, 2, 3, 4},
		})
	}

	require.EqualError(t, err, "NALU count (51) exceeds maximum allowed (50)")
}

func TestDecodeErrorMissingPacket(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	_, err = d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17645,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x1c, 0x85, 0x01, 0x02},
	})
	require.Equal(t, ErrMorePacketsNeeded, err)

	_, err = d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17647,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x1c, 0x05, 0x01, 0x02},
	})
	require.EqualError(t, err, "discarding frame since a RTP packet is missing")
}

func serializePackets(packets []*rtp.Packet) ([]byte, error) {
	var buf []byte

	for _, pkt := range packets {
		buf2, err := pkt.Marshal()
		if err != nil {
			return nil, err
		}

		tmp := make([]byte, 4)
		binary.LittleEndian.PutUint32(tmp, uint32(len(buf2)))
		buf = append(buf, tmp...)
		buf = append(buf, buf2...)
	}

	return buf, nil
}

func unserializePackets(data []byte) ([]*rtp.Packet, error) {
	var packets []*rtp.Packet
	buf := data

	for {
		if len(buf) < 4 {
			return nil, errors.New("not enough bits")
		}

		size := binary.LittleEndian.Uint32(buf[:4])
		buf = buf[4:]

		if uint32(len(buf)) < size {
			return nil, errors.New("not enough bits")
		}

		var pkt rtp.Packet
		err := pkt.Unmarshal(buf[:size])
		if err != nil {
			return nil, err
		}

		packets = append(packets, &pkt)
		buf = buf[size:]

		if len(buf) == 0 {
			break
		}
	}

	return packets, nil
}

func FuzzDecoder(f *testing.F) {
	for _, ca := range cases {
		buf, err := serializePackets(ca.pkts)
		if err != nil {
			panic(err)
		}
		f.Add(buf)
	}

	for _, ca := range casesDecodeOnly {
		buf, err := serializePackets(ca.pkts)
		if err != nil {
			panic(err)
		}
		f.Add(buf)
	}

	f.Fuzz(func(t *testing.T, buf []byte) {
		packets, err := unserializePackets(buf)
		if err != nil {
			t.Skip()
			return
		}

		d := &Decoder{}
		err = d.Init()
		require.NoError(t, err)

		for _, pkt := range packets {
			if au, err2 := d.Decode(pkt); err2 == nil {
				if len(au) == 0 {
					t.Errorf("should not happen")
				}

				for _, nalu := range au {
					if len(nalu) == 0 {
						t.Errorf("should not happen")
					}
				}
			}
		}
	})
}
