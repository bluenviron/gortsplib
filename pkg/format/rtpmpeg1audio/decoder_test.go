package rtpmpeg1audio

import (
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

			var frames [][]byte

			for _, pkt := range ca.pkts {
				frames, err = d.Decode(pkt)
			}

			require.NoError(t, err)
			require.Equal(t, ca.frames, frames)
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
			var frames [][]byte
			frames, err = d.Decode(pkt)
			if err != nil {
				continue
			}

			require.NotEmpty(t, frames)

			for _, frame := range frames {
				require.NotEmpty(t, frame)
			}

			e := &Encoder{
				SSRC:                  ptrOf(uint32(12321)),
				InitialSequenceNumber: ptrOf(uint16(45432)),
			}
			err = e.Init()
			require.NoError(t, err)

			e.Encode(frames) //nolint:errcheck
		}
	})
}
