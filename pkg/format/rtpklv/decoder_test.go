package rtpklv

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecoder_SinglePacketKLV(t *testing.T) {
	// Create a simple KLV item: 16-byte Universal Label + 1-byte length + value
	klvData := make([]byte, 0, 100)
	// Universal Label Key (16 bytes) - starts with 0x060e2b34
	klvData = append(klvData, 0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01)
	// Length (1 byte) - short form, value length = 5
	klvData = append(klvData, 0x05)
	// Value (5 bytes)
	klvData = append(klvData, 0x48, 0x65, 0x6c, 0x6c, 0x6f) // "Hello"

	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	pkt := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 1,
			Timestamp:      1000,
			Marker:         true, // Single packet
		},
		Payload: klvData,
	}

	result, err := d.Decode(pkt)
	require.NoError(t, err)
	require.Equal(t, klvData, result)
}

func TestDecoder_MultiPacketKLV(t *testing.T) {
	// Create a larger KLV item that will be split across packets
	klvData := make([]byte, 0, 200)
	// Universal Label Key (16 bytes)
	klvData = append(klvData, 0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
		0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01)
	// Length (2 bytes) - long form, value length = 100
	klvData = append(klvData, 0x81, 0x64) // 0x81 = long form with 1 byte, 0x64 = 100
	// Value (100 bytes)
	for i := 0; i < 100; i++ {
		klvData = append(klvData, byte(i))
	}

	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	// Split into 3 packets
	packet1 := klvData[:50]
	packet2 := klvData[50:100]
	packet3 := klvData[100:]

	// First packet
	pkt1 := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 1,
			Timestamp:      2000,
			Marker:         false,
		},
		Payload: packet1,
	}

	result, err := d.Decode(pkt1)
	require.Equal(t, ErrMorePacketsNeeded, err)
	require.Nil(t, result)

	// Second packet
	pkt2 := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 2,
			Timestamp:      2000, // Same timestamp
			Marker:         false,
		},
		Payload: packet2,
	}

	result, err = d.Decode(pkt2)
	require.Equal(t, ErrMorePacketsNeeded, err)
	require.Nil(t, result)

	// Third packet (last)
	pkt3 := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 3,
			Timestamp:      2000, // Same timestamp
			Marker:         true, // Last packet
		},
		Payload: packet3,
	}

	result, err = d.Decode(pkt3)
	require.NoError(t, err)
	require.Equal(t, klvData, result)
}

func TestDecoder_PacketLoss(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	// First packet
	pkt1 := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 1,
			Timestamp:      3000,
			Marker:         false,
		},
		Payload: []byte{0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01},
	}

	result, err := d.Decode(pkt1)
	require.Equal(t, ErrMorePacketsNeeded, err)
	require.Nil(t, result)

	// Skip packet 2 (simulate packet loss)
	// Third packet
	pkt3 := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 3, // Gap in sequence
			Timestamp:      3000,
			Marker:         true,
		},
		Payload: []byte{0x01, 0x01, 0x01, 0x01, 0x05, 0x48, 0x65, 0x6c, 0x6c, 0x6f},
	}

	result, err = d.Decode(pkt3)
	require.Error(t, err)
	require.Contains(t, err.Error(), "packet loss detected")
	require.Nil(t, result)
}

func TestDecoder_NonStartingPacket(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	// Packet that doesn't start with KLV Universal Label
	pkt := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 1,
			Timestamp:      4000,
			Marker:         false,
		},
		Payload: []byte{0x01, 0x02, 0x03, 0x04}, // Not a KLV start
	}

	result, err := d.Decode(pkt)
	require.Equal(t, ErrNonStartingPacketAndNoPrevious, err)
	require.Nil(t, result)
}

func TestDecoder_TimestampChange(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	// First packet of a KLV unit
	pkt1 := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 1,
			Timestamp:      5000,
			Marker:         false,
		},
		Payload: []byte{0x06, 0x0e, 0x2b, 0x34, 0x01, 0x01, 0x01, 0x01,
			0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x05},
	}

	result, err := d.Decode(pkt1)
	require.Equal(t, ErrMorePacketsNeeded, err)
	require.Nil(t, result)

	// Second packet with different timestamp (new KLV unit)
	pkt2 := &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: 2,
			Timestamp:      6000, // Different timestamp
			Marker:         true,
		},
		Payload: []byte{0x48, 0x65, 0x6c, 0x6c, 0x6f},
	}

	result, err = d.Decode(pkt2)
	require.Error(t, err)
	require.Contains(t, err.Error(), "incomplete KLV unit: timestamp changed")
	require.Nil(t, result)
}

func TestParseKLVLength(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		offset     int
		wantLength int
		wantSize   int
		wantErr    bool
	}{
		{
			name:       "short form",
			data:       []byte{0x05}, // Length = 5
			offset:     0,
			wantLength: 5,
			wantSize:   1,
			wantErr:    false,
		},
		{
			name:       "long form 1 byte",
			data:       []byte{0x81, 0x64}, // Length = 100
			offset:     0,
			wantLength: 100,
			wantSize:   2,
			wantErr:    false,
		},
		{
			name:       "long form 2 bytes",
			data:       []byte{0x82, 0x01, 0x00}, // Length = 256
			offset:     0,
			wantLength: 256,
			wantSize:   3,
			wantErr:    false,
		},
		{
			name:    "insufficient data",
			data:    []byte{},
			offset:  0,
			wantErr: true,
		},
		{
			name:    "indefinite length",
			data:    []byte{0x80},
			offset:  0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			length, size, err := parseKLVLength(tt.data, tt.offset)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantLength, length)
				require.Equal(t, tt.wantSize, size)
			}
		})
	}
}

func TestIsKLVStart(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{
			name:    "valid KLV start",
			payload: []byte{0x06, 0x0e, 0x2b, 0x34, 0x01, 0x02, 0x03},
			want:    true,
		},
		{
			name:    "invalid KLV start",
			payload: []byte{0x01, 0x02, 0x03, 0x04},
			want:    false,
		},
		{
			name:    "too short",
			payload: []byte{0x06, 0x0e, 0x2b},
			want:    false,
		},
		{
			name:    "empty",
			payload: []byte{},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isKLVStart(tt.payload)
			require.Equal(t, tt.want, got)
		})
	}
}
