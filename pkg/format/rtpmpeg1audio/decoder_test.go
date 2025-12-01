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
			d := &Decoder{}
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

		d := &Decoder{}
		err = d.Init()
		require.NoError(t, err)

		for _, pkt := range packets {
			if frames, err2 := d.Decode(pkt); err2 == nil {
				if len(frames) == 0 {
					t.Errorf("should not happen")
				}

				for _, frame := range frames {
					if len(frame) == 0 {
						t.Errorf("should not happen")
					}
				}
			}
		}
	})
}
