package rtpklv

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
			var d Decoder
			err := d.Init()
			require.NoError(t, err)

			var klvUnit []byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				var addUnits []byte
				addUnits, err = d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if errors.Is(err, ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				klvUnit = append(klvUnit, addUnits...)
			}

			require.Equal(t, ca.klvUnit, klvUnit)
		})
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

	f.Fuzz(func(t *testing.T, buf []byte) {
		packets, err := unserializePackets(buf)
		if err != nil {
			t.Skip()
			return
		}

		var d Decoder
		err = d.Init()
		require.NoError(t, err)

		for _, pkt := range packets {
			var unit []byte
			unit, err = d.Decode(pkt)
			if err != nil {
				continue
			}

			require.NotEmpty(t, unit)

			e := &Encoder{
				SSRC:                  new(uint32(12321)),
				InitialSequenceNumber: new(uint16(45432)),
			}
			err = e.Init()
			require.NoError(t, err)

			e.Encode(unit) //nolint:errcheck
		}
	})
}

func TestDecodeUnitsAreNotReused(t *testing.T) {
	klvUnit := func(fill byte, size int) []byte {
		return append([]byte{
			0x06, 0x0e, 0x2b, 0x34, 0x02, 0x0b, 0x01, 0x01,
			0x0e, 0x01, 0x03, 0x01, 0x01, 0x00, 0x00, 0x00,
			byte(size),
		}, bytes.Repeat([]byte{fill}, size)...)
	}

	for _, ca := range []struct {
		name   string
		second []byte
	}{
		{"same size", klvUnit(0xbb, 20)},
		{"shorter", klvUnit(0xbb, 8)},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var d Decoder
			err := d.Init()
			require.NoError(t, err)

			first := klvUnit(0xaa, 20)

			decoded, err := d.Decode(&rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: first,
			})
			require.NoError(t, err)
			require.Equal(t, first, decoded)

			_, err = d.Decode(&rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					SequenceNumber: 17646,
					Timestamp:      2289529357,
					SSRC:           0x9dbb7812,
				},
				Payload: ca.second,
			})
			require.NoError(t, err)

			// the unit returned by the first Decode() must not be touched
			// by the second one.
			require.Equal(t, first, decoded)
		})
	}
}
