package rtpmpegts

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	for _, ca := range []struct {
		name       string
		tsData     []byte
		maxSize    int
		pktCount   int
		firstSeqNo uint16
	}{
		{
			"single TS packet",
			makeTSPacket(0x01),
			1316,
			1,
			0x44ed,
		},
		{
			"seven TS packets exact fit",
			bytes.Repeat(makeTSPacket(0x01), 7),
			1316,
			1,
			0x44ed,
		},
		{
			"eight TS packets splits into two",
			bytes.Repeat(makeTSPacket(0x01), 8),
			1316,
			2,
			0x44ed,
		},
		{
			"fourteen TS packets splits into two",
			bytes.Repeat(makeTSPacket(0x01), 14),
			1316,
			2,
			0x44ed,
		},
		{
			"fifteen TS packets splits into three",
			bytes.Repeat(makeTSPacket(0x01), 15),
			1316,
			3,
			0x44ed,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				SSRC:                  ptrOf(uint32(0x9dbb7812)),
				InitialSequenceNumber: ptrOf(ca.firstSeqNo),
				PayloadMaxSize:        ca.maxSize,
			}
			err := e.Init()
			require.NoError(t, err)

			pkts, err := e.Encode(ca.tsData)
			require.NoError(t, err)
			require.Equal(t, ca.pktCount, len(pkts))

			// verify all packets have correct header fields
			for i, pkt := range pkts {
				require.Equal(t, uint8(rtpVersion), pkt.Version)
				require.Equal(t, uint8(payloadType), pkt.PayloadType)
				require.Equal(t, uint32(0x9dbb7812), pkt.SSRC)
				require.Equal(t, ca.firstSeqNo+uint16(i), pkt.SequenceNumber)
			}

			// verify payload reassembles to original
			var reassembled []byte
			for _, pkt := range pkts {
				reassembled = append(reassembled, pkt.Payload...)
			}
			require.Equal(t, ca.tsData, reassembled)
		})
	}
}

func TestEncodeSequenceNumberIncrement(t *testing.T) {
	e := &Encoder{
		SSRC:                  ptrOf(uint32(0x12345678)),
		InitialSequenceNumber: ptrOf(uint16(100)),
		PayloadMaxSize:        1316,
	}
	err := e.Init()
	require.NoError(t, err)

	// first encode: 8 TS packets -> 2 RTP packets (seq 100, 101)
	pkts1, err := e.Encode(bytes.Repeat(makeTSPacket(0x01), 8))
	require.NoError(t, err)
	require.Equal(t, uint16(100), pkts1[0].SequenceNumber)
	require.Equal(t, uint16(101), pkts1[1].SequenceNumber)

	// second encode: 1 TS packet -> 1 RTP packet (seq 102)
	pkts2, err := e.Encode(makeTSPacket(0x02))
	require.NoError(t, err)
	require.Equal(t, uint16(102), pkts2[0].SequenceNumber)
}

func TestEncodeErrorEmpty(t *testing.T) {
	e := &Encoder{
		SSRC:                  ptrOf(uint32(0x12345678)),
		InitialSequenceNumber: ptrOf(uint16(0)),
	}
	err := e.Init()
	require.NoError(t, err)

	_, err = e.Encode([]byte{})
	require.EqualError(t, err, "tsData is empty")
}

func TestEncodeErrorNotAligned(t *testing.T) {
	e := &Encoder{
		SSRC:                  ptrOf(uint32(0x12345678)),
		InitialSequenceNumber: ptrOf(uint16(0)),
	}
	err := e.Init()
	require.NoError(t, err)

	_, err = e.Encode(make([]byte, 100))
	require.EqualError(t, err, "tsData length 100 is not a multiple of 188")
}

func TestEncodeInitErrorBadMaxSize(t *testing.T) {
	e := &Encoder{
		PayloadMaxSize: 1000,
	}
	err := e.Init()
	require.EqualError(t, err, "PayloadMaxSize 1000 is not a multiple of 188")
}

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{}
	err := e.Init()
	require.NoError(t, err)
	require.NotNil(t, e.SSRC)
	require.NotNil(t, e.InitialSequenceNumber)
	require.Equal(t, defaultPayloadMaxSize, e.PayloadMaxSize)
}

func TestRoundTrip(t *testing.T) {
	e := &Encoder{
		SSRC:                  ptrOf(uint32(0xAABBCCDD)),
		InitialSequenceNumber: ptrOf(uint16(0)),
		PayloadMaxSize:        376, // 2 TS packets per RTP packet
	}
	err := e.Init()
	require.NoError(t, err)

	d := &Decoder{}
	err = d.Init()
	require.NoError(t, err)

	// 5 TS packets -> 3 RTP packets (2+2+1)
	original := bytes.Repeat(makeTSPacket(0x42), 5)
	pkts, err := e.Encode(original)
	require.NoError(t, err)
	require.Equal(t, 3, len(pkts))

	var reassembled []byte
	for _, pkt := range pkts {
		decoded, err := d.Decode(pkt)
		require.NoError(t, err)
		reassembled = append(reassembled, decoded...)
	}
	require.Equal(t, original, reassembled)
}
