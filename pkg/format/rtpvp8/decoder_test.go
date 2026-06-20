package rtpvp8

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
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var frame []byte

			for _, pkt := range ca.pkts {
				frame, err = d.Decode(pkt)
			}

			require.NoError(t, err)
			require.Equal(t, ca.frame, frame)
		})
	}
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
		Payload: mergeBytes([]byte{0x10}, bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 364), []byte{0x01, 0x02, 0x03}),
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
		Payload: mergeBytes([]byte{0x00, 0x04}, bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 364), []byte{0x01, 0x02}),
	})
	require.EqualError(t, err, "discarding frame since a RTP packet is missing")
}

func TestDecodeMultiplePartitions(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	frame := mergeBytes([]byte{1, 2, 3}, []byte{4, 5}, []byte{6, 7, 8, 9})

	pkts := []*rtp.Packet{
		{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x10, 1, 2, 3},
		},
		{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17646,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x11, 4, 5},
		},
		{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17647,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x12, 6, 7, 8, 9},
		},
	}

	var out []byte
	for _, pkt := range pkts {
		out, err = d.Decode(pkt)
	}

	require.NoError(t, err)
	require.Equal(t, frame, out)
}

func TestDecodeFragmentedMultiplePartitions(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	frame := mergeBytes([]byte{1, 2, 3}, []byte{4, 5}, []byte{6, 7, 8, 9})

	pkts := []*rtp.Packet{
		{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x10, 1, 2, 3},
		},
		{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17646,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x11, 4, 5},
		},
		{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17647,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x02, 6, 7},
		},
		{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17648,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x02, 8, 9},
		},
	}

	var out []byte
	for _, pkt := range pkts {
		out, err = d.Decode(pkt)
	}

	require.NoError(t, err)
	require.Equal(t, frame, out)
}

func TestDecodeFrameRestart(t *testing.T) {
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
		Payload: []byte{0x10, 1, 2, 3},
	})
	require.Equal(t, ErrMorePacketsNeeded, err)

	_, err = d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17646,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x00, 4, 5},
	})
	require.Equal(t, ErrMorePacketsNeeded, err)

	frame, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17647,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x10, 6, 7, 8},
	})
	require.NoError(t, err)
	require.Equal(t, []byte{6, 7, 8}, frame)
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
			if frame, err2 := d.Decode(pkt); err2 == nil {
				if len(frame) == 0 {
					t.Errorf("should not happen")
				}
			}
		}
	})
}
