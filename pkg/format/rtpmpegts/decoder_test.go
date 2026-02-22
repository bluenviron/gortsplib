package rtpmpegts

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func makeTSPacket(pid byte) []byte {
	pkt := make([]byte, MPEGTSPacketSize)
	pkt[0] = SyncByte
	pkt[1] = 0x00
	pkt[2] = pid
	return pkt
}

func TestDecode(t *testing.T) {
	for _, ca := range []struct {
		name    string
		payload []byte
	}{
		{
			"single TS packet",
			makeTSPacket(0x01),
		},
		{
			"two TS packets",
			append(makeTSPacket(0x01), makeTSPacket(0x02)...),
		},
		{
			"seven TS packets",
			bytes.Repeat(makeTSPacket(0x01), 7),
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			result, err := d.Decode(&rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    33,
					SequenceNumber: 1000,
					SSRC:           0x12345678,
				},
				Payload: ca.payload,
			})
			require.NoError(t, err)
			require.Equal(t, ca.payload, result)
		})
	}
}

func TestDecodeErrorEmpty(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	_, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{Version: 2},
		Payload: []byte{},
	})
	require.EqualError(t, err, "empty MPEG-TS payload")
}

func TestDecodeErrorNotAligned(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	_, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{Version: 2},
		Payload: make([]byte, 100),
	})
	require.EqualError(t, err, "payload length 100 is not a multiple of 188")
}

func TestDecodeErrorMissingSyncByteFirst(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	payload := make([]byte, MPEGTSPacketSize)
	payload[0] = 0x00 // wrong sync byte

	_, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{Version: 2},
		Payload: payload,
	})
	require.EqualError(t, err, "missing sync byte at offset 0: got 0x00")
}

func TestDecodeErrorMissingSyncByteSecond(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	payload := append(makeTSPacket(0x01), make([]byte, MPEGTSPacketSize)...)
	payload[MPEGTSPacketSize] = 0xFF // wrong sync byte on second packet

	_, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{Version: 2},
		Payload: payload,
	})
	require.EqualError(t, err, "missing sync byte at offset 188: got 0xff")
}
