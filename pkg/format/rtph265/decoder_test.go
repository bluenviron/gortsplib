package rtph265

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"testing"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h265"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var nalus [][]byte

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
				nalus = append(nalus, addNALUs...)
			}

			require.Equal(t, ca.nalus, nalus)
		})
	}
}

func TestDecoderErrorNALUSize(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	size := 0
	i := uint16(0)

	for size < h265.MaxAccessUnitSize {
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
				[]byte{byte(h265.NALUType_FragmentationUnit << 1), 0, flags},
				bytes.Repeat([]byte{1, 2, 3, 4}, 1400/4)...,
			),
		})

		size += 1400
		i++
	}

	require.EqualError(t, err, "NALU size (8388802) is too big, maximum is 8388608")
}

func TestDecoderErrorNALUCount(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	for i := 0; i <= h265.MaxNALUsPerAccessUnit; i++ {
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

	require.EqualError(t, err, "NALU count (22) exceeds maximum allowed (21)")
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
		Payload: []byte{0x63, 0x02, 0x80, 0x03, 0x04},
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
		Payload: []byte{0x63, 0x02, 0x00, 0x04},
	})
	require.EqualError(t, err, "discarding frame since a RTP packet is missing")
}

func TestDecodeMultipleNALUsInFU(t *testing.T) {
	data, err := os.ReadFile("testdata/multiple-nalus-in-fu.rtp")
	require.NoError(t, err)

	pkts, err := unserializePackets(data)
	require.NoError(t, err)
	require.Equal(t, 2, len(pkts), "must be 2 packets")

	d := &Decoder{}
	err = d.Init()
	require.NoError(t, err)

	_, err = d.Decode(pkts[0])
	require.Equal(t, ErrMorePacketsNeeded, err)

	nalus, err := d.Decode(pkts[1])
	require.NoError(t, err)
	require.Equal(t, 4, len(nalus), "must be 4 NALUs")

	expectedNALUs := []struct {
		typ  h265.NALUType
		size int
	}{
		{h265.NALUType_VPS_NUT, 24},
		{h265.NALUType_SPS_NUT, 42},
		{h265.NALUType_PPS_NUT, 7},
		{h265.NALUType_IDR_W_RADL, 2715},
	}

	for i, nalu := range nalus {
		typ := h265.NALUType((nalu[0] >> 1) & 0b111111)
		require.Equal(t, expectedNALUs[i].typ, typ, "nalu types don't match")
		require.Equal(t, expectedNALUs[i].size, len(nalu), "nalu sizes don't match")
	}
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

	f.Add([]byte{
		0x11, 0x00, 0x00, 0x00, 0x80, 0x60, 0x03, 0xe8,
		0x00, 0x00, 0x04, 0xd2, 0x12, 0x34, 0x56, 0x78,
		0x62, 0x00, 0x81, 0x01, 0x02, 0x10, 0x00, 0x00,
		0x00, 0x80, 0x60, 0x03, 0xe9, 0x00, 0x00, 0x04,
		0xd2, 0x12, 0x34, 0x56, 0x78, 0x02, 0x01, 0xaa,
		0xbb, 0x11, 0x00, 0x00, 0x00, 0x80, 0x60, 0x03,
		0xea, 0x00, 0x00, 0x04, 0xd2, 0x12, 0x34, 0x56,
		0x78, 0x62, 0x00, 0x01, 0x03, 0x04,
	})

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
