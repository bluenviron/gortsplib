package rtpklv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func uint32Ptr(v uint32) *uint32 {
	return &v
}

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func TestEncoder_EncodeMultiple(t *testing.T) {
	// Create multiple KLV items
	klvItem1 := []byte{
		0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x03,             // Length = 3
		0x41, 0x42, 0x43, // "ABC"
	}

	klvItem2 := []byte{
		0x06, 0x0e, 0x2b, 0x34, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x03,             // Length = 3
		0x44, 0x45, 0x46, // "DEF"
	}

	e := &Encoder{
		PayloadType:           96,
		SSRC:                  uint32Ptr(0x87654321),
		InitialSequenceNumber: uint16Ptr(3000),
	}
	err := e.Init()
	require.NoError(t, err)

	packets, err := e.Encode([][]byte{klvItem1, klvItem2})
	require.NoError(t, err)

	// Should fit in a single packet
	require.Len(t, packets, 1)

	pkt := packets[0]
	require.True(t, pkt.Marker)

	// Payload should be concatenation of both items
	expectedPayload := make([]byte, 0, len(klvItem1)+len(klvItem2))
	expectedPayload = append(expectedPayload, klvItem1...)
	expectedPayload = append(expectedPayload, klvItem2...)
	require.Equal(t, expectedPayload, pkt.Payload)
}

func TestEncoder_RandomInitialState(t *testing.T) {
	e := &Encoder{
		PayloadType: 96,
	}
	err := e.Init()
	require.NoError(t, err)

	// Should have generated random values
	require.NotNil(t, e.SSRC)
	require.NotNil(t, e.InitialSequenceNumber)
	require.Equal(t, defaultPayloadMaxSize, e.PayloadMaxSize)
}

func TestEncoder_LargeFragmentation(t *testing.T) {
	// Create a very large KLV unit to test fragmentation logic
	largeValue := make([]byte, 10000)
	for i := range largeValue {
		largeValue[i] = byte(i % 256)
	}

	klvUnit := make([]byte, 0, 10020)
	// Universal Label Key (16 bytes)
	klvUnit = append(klvUnit, 0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01)
	// Length (3 bytes) - long form
	klvUnit = append(klvUnit, 0x82, 0x27, 0x10) // Length = 10000
	// Value
	klvUnit = append(klvUnit, largeValue...)

	e := &Encoder{
		PayloadType:           96,
		SSRC:                  uint32Ptr(0x22222222),
		InitialSequenceNumber: uint16Ptr(500),
		PayloadMaxSize:        1000,
	}
	err := e.Init()
	require.NoError(t, err)

	packets, err := e.Encode([][]byte{klvUnit})
	require.NoError(t, err)

	// Should be fragmented into multiple packets
	expectedPackets := (len(klvUnit) + 999) / 1000 // Ceiling division
	require.Equal(t, expectedPackets, len(packets))

	// Verify packet properties
	for i, pkt := range packets {
		require.Equal(t, uint8(96), pkt.PayloadType)
		require.Equal(t, uint32(0x22222222), pkt.SSRC)
		require.Equal(t, uint16(500+i), pkt.SequenceNumber)

		// Check marker bit
		if i == len(packets)-1 {
			require.True(t, pkt.Marker)
		} else {
			require.False(t, pkt.Marker)
		}

		// Check payload size
		if i == len(packets)-1 {
			// Last packet might be smaller
			require.LessOrEqual(t, len(pkt.Payload), 1000)
		} else {
			require.Equal(t, 1000, len(pkt.Payload))
		}
	}

	// Reconstruct and verify
	var reconstructed []byte
	for _, pkt := range packets {
		reconstructed = append(reconstructed, pkt.Payload...)
	}
	require.Equal(t, klvUnit, reconstructed)
}
