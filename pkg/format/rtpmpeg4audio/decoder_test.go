package rtpmpeg4audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				SizeLength:       ca.sizeLength,
				IndexLength:      ca.indexLength,
				IndexDeltaLength: ca.indexDeltaLength,
			}
			err := d.Init()
			require.NoError(t, err)

			var aus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				var addAUs [][]byte
				addAUs, err = d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if errors.Is(err, ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				aus = append(aus, addAUs...)
			}

			require.Equal(t, ca.aus, aus)
		})
	}
}

func TestDecodeADTS(t *testing.T) {
	d := &Decoder{
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	err := d.Init()
	require.NoError(t, err)

	for range 2 {
		var aus [][]byte
		aus, err = d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17645,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{
				0x00, 0x10, 0x00, 0x09 << 3,
				0xff, 0xf1, 0x4c, 0x80, 0x1, 0x3f, 0xfc, 0xaa, 0xbb,
			},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{{0xaa, 0xbb}}, aus)
	}
}

func TestDecodeErrorMissingPacket(t *testing.T) {
	d := &Decoder{
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
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
		Payload: mergeBytes(
			[]byte{0x0, 0x10, 0x2d, 0x80},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
		),
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
		Payload: mergeBytes(
			[]byte{0x00, 0x10, 0x2d, 0x80},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
		),
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

	f.Fuzz(func(t *testing.T, buf []byte) {
		packets, err := unserializePackets(buf)
		if err != nil {
			t.Skip()
			return
		}

		d := &Decoder{
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		}
		err = d.Init()
		require.NoError(t, err)

		for _, pkt := range packets {
			if aus, err2 := d.Decode(pkt); err2 == nil {
				if len(aus) == 0 {
					t.Errorf("should not happen")
				}

				for _, au := range aus {
					if len(au) == 0 {
						t.Errorf("should not happen")
					}
				}
			}
		}
	})
}
