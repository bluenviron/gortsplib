package rtpmpegts

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

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

func makeTSPacket(pid byte) []byte {
	pkt := make([]byte, mpegtsPacketSize)
	pkt[0] = syncByte
	pkt[1] = 0x00
	pkt[2] = pid
	return pkt
}

var cases = []struct {
	name string
	ts   [][]byte
	rtp  []*rtp.Packet
}{
	{
		"single TS packet",
		[][]byte{makeTSPacket(0x01)},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    33,
					SequenceNumber: 1000,
					SSRC:           0x12345678,
				},
				Payload: makeTSPacket(0x01),
			},
		},
	},
	{
		"two TS packets",
		[][]byte{makeTSPacket(0x01), makeTSPacket(0x02)},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    33,
					SequenceNumber: 1000,
					SSRC:           0x12345678,
				},
				Payload: append(makeTSPacket(0x01), makeTSPacket(0x02)...),
			},
		},
	},
	{
		"seven TS packets",
		[][]byte{
			makeTSPacket(0x01), makeTSPacket(0x02), makeTSPacket(0x03), makeTSPacket(0x04),
			makeTSPacket(0x05), makeTSPacket(0x06), makeTSPacket(0x07),
		},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    33,
					SequenceNumber: 1000,
					SSRC:           0x12345678,
				},
				Payload: mergeBytes(
					makeTSPacket(0x01),
					makeTSPacket(0x02),
					makeTSPacket(0x03),
					makeTSPacket(0x04),
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    33,
					SequenceNumber: 1001,
					SSRC:           0x12345678,
				},
				Payload: mergeBytes(
					makeTSPacket(0x05),
					makeTSPacket(0x06),
					makeTSPacket(0x07),
				),
			},
		},
	},
}

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var ts [][]byte

			for _, pkt := range ca.rtp {
				var partialTS [][]byte
				partialTS, err = d.Decode(pkt)
				require.NoError(t, err)
				ts = append(ts, partialTS...)
			}

			require.Equal(t, ca.ts, ts)
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
		buf, err := serializePackets(ca.rtp)
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
			if ts, err2 := d.Decode(pkt); err2 == nil {
				if len(ts) == 0 {
					t.Errorf("should not happen")
				}

				for _, tsPacket := range ts {
					if len(tsPacket) == 0 {
						t.Errorf("should not happen")
					}
				}
			}
		}
	})
}
