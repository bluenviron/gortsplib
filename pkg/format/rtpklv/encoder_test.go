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

func TestEncoder_SinglePacket(t *testing.T) {
	// Create a small KLV unit that fits in one packet
	klvUnit := make([]byte, 0, 50)
	// Universal Label Key (16 bytes)
	klvUnit = append(klvUnit, 0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01)
	// Length (1 byte)
	klvUnit = append(klvUnit, 0x05)
	// Value (5 bytes)
	klvUnit = append(klvUnit, 0x48, 0x65, 0x6c, 0x6c, 0x6f) // "Hello"

	e := &Encoder{
		PayloadType:           96,
		SSRC:                  uint32Ptr(0x12345678),
		InitialSequenceNumber: uint16Ptr(1000),
		PayloadMaxSize:        1460,
	}
	err := e.Init()
	require.NoError(t, err)

	packets, err := e.Encode(klvUnit, 90000)
	require.NoError(t, err)
	require.Len(t, packets, 1)

	pkt := packets[0]
	require.Equal(t, uint8(96), pkt.PayloadType)
	require.Equal(t, uint32(0x12345678), pkt.SSRC)
	require.Equal(t, uint16(1000), pkt.SequenceNumber)
	require.Equal(t, uint32(90000), pkt.Timestamp)
	require.True(t, pkt.Marker) // Single packet should have marker bit set
	require.Equal(t, klvUnit, pkt.Payload)
}

func TestEncoder_MultiplePackets(t *testing.T) {
	// Create a large KLV unit that needs to be fragmented
	klvUnit := make([]byte, 0, 3000)
	// Universal Label Key (16 bytes)
	klvUnit = append(klvUnit, 0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01)
	// Length (2 bytes) - long form for large value
	klvUnit = append(klvUnit, 0x82, 0x0b, 0xb8) // Length = 3000
	// Value (3000 bytes)
	for i := 0; i < 3000; i++ {
		klvUnit = append(klvUnit, byte(i%256))
	}

	e := &Encoder{
		PayloadType:           96,
		SSRC:                  uint32Ptr(0x12345678),
		InitialSequenceNumber: uint16Ptr(2000),
		PayloadMaxSize:        1000, // Force fragmentation
	}
	err := e.Init()
	require.NoError(t, err)

	packets, err := e.Encode(klvUnit, 180000)
	require.NoError(t, err)
	require.Greater(t, len(packets), 1) // Should be fragmented

	// Check that all packets have the same timestamp and SSRC
	for i, pkt := range packets {
		require.Equal(t, uint8(96), pkt.PayloadType)
		require.Equal(t, uint32(0x12345678), pkt.SSRC)
		require.Equal(t, uint16(2000+i), pkt.SequenceNumber)
		require.Equal(t, uint32(180000), pkt.Timestamp)

		// Only the last packet should have marker bit set
		if i == len(packets)-1 {
			require.True(t, pkt.Marker, "Last packet should have marker bit set")
		} else {
			require.False(t, pkt.Marker, "Non-last packet should not have marker bit set")
		}
	}

	// Reconstruct the original KLV unit
	var reconstructed []byte
	for _, pkt := range packets {
		reconstructed = append(reconstructed, pkt.Payload...)
	}
	require.Equal(t, klvUnit, reconstructed)
}

func TestEncoder_EncodeMultiple(t *testing.T) {
	// Create multiple KLV items
	klvItem1 := []byte{
		0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x03, // Length = 3
		0x41, 0x42, 0x43, // "ABC"
	}

	klvItem2 := []byte{
		0x06, 0x0e, 0x2b, 0x34, 0x02, 0x02, 0x02, 0x02,
		0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02, 0x02,
		0x03, // Length = 3
		0x44, 0x45, 0x46, // "DEF"
	}

	e := &Encoder{
		PayloadType:           96,
		SSRC:                  uint32Ptr(0x87654321),
		InitialSequenceNumber: uint16Ptr(3000),
	}
	err := e.Init()
	require.NoError(t, err)

	packets, err := e.EncodeMultiple([][]byte{klvItem1, klvItem2}, 270000)
	require.NoError(t, err)

	// Should fit in a single packet
	require.Len(t, packets, 1)

	pkt := packets[0]
	require.Equal(t, uint32(270000), pkt.Timestamp)
	require.True(t, pkt.Marker)

	// Payload should be concatenation of both items
	expectedPayload := make([]byte, 0, len(klvItem1)+len(klvItem2))
	expectedPayload = append(expectedPayload, klvItem1...)
	expectedPayload = append(expectedPayload, klvItem2...)
	require.Equal(t, expectedPayload, pkt.Payload)
}

func TestEncoder_EmptyKLVUnit(t *testing.T) {
	e := &Encoder{
		PayloadType: 96,
	}
	err := e.Init()
	require.NoError(t, err)

	packets, err := e.Encode([]byte{}, 360000)
	require.Error(t, err)
	require.Contains(t, err.Error(), "KLV unit is empty")
	require.Nil(t, packets)
}

func TestEncoder_EmptyKLVItems(t *testing.T) {
	e := &Encoder{
		PayloadType: 96,
	}
	err := e.Init()
	require.NoError(t, err)

	packets, err := e.EncodeMultiple([][]byte{}, 360000)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no KLV items provided")
	require.Nil(t, packets)
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

func TestEncoder_SequenceNumberIncrement(t *testing.T) {
	klvUnit := []byte{
		0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01,
		0x05, // Length = 5
		0x48, 0x65, 0x6c, 0x6c, 0x6f, // "Hello"
	}

	e := &Encoder{
		PayloadType:           96,
		SSRC:                  uint32Ptr(0x11111111),
		InitialSequenceNumber: uint16Ptr(100),
	}
	err := e.Init()
	require.NoError(t, err)

	// Encode first KLV unit
	packets1, err := e.Encode(klvUnit, 450000)
	require.NoError(t, err)
	require.Len(t, packets1, 1)
	require.Equal(t, uint16(100), packets1[0].SequenceNumber)

	// Encode second KLV unit
	packets2, err := e.Encode(klvUnit, 540000)
	require.NoError(t, err)
	require.Len(t, packets2, 1)
	require.Equal(t, uint16(101), packets2[0].SequenceNumber)
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

	packets, err := e.Encode(klvUnit, 630000)
	require.NoError(t, err)

	// Should be fragmented into multiple packets
	expectedPackets := (len(klvUnit) + 999) / 1000 // Ceiling division
	require.Equal(t, expectedPackets, len(packets))

	// Verify packet properties
	for i, pkt := range packets {
		require.Equal(t, uint8(96), pkt.PayloadType)
		require.Equal(t, uint32(0x22222222), pkt.SSRC)
		require.Equal(t, uint16(500+i), pkt.SequenceNumber)
		require.Equal(t, uint32(630000), pkt.Timestamp)

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
