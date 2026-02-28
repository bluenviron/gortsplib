package rtpmpegts

import (
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
	pkt := make([]byte, MPEGTSPacketSize)
	pkt[0] = SyncByte
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
		[][]byte{makeTSPacket(0x01), makeTSPacket(0x02), makeTSPacket(0x03), makeTSPacket(0x04),
			makeTSPacket(0x05), makeTSPacket(0x06), makeTSPacket(0x07)},
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
				partialTS, err := d.Decode(pkt)
				require.NoError(t, err)
				ts = append(ts, partialTS...)
			}

			require.Equal(t, ca.ts, ts)
		})
	}
}

/*func TestDecodeErrorEmpty(t *testing.T) {
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
}*/
