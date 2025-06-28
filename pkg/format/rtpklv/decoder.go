package rtpklv

import (
	"errors"
	"fmt"

	"github.com/pion/rtp"
)

// ErrMorePacketsNeeded is returned when more packets are needed to complete a KLV unit.
var ErrMorePacketsNeeded = errors.New("need more packets")

// ErrNonStartingPacketAndNoPrevious is returned when we received a non-starting
// packet of a fragmented KLV unit and we didn't receive anything before.
// It's normal to receive this when decoding a stream that has been already
// running for some time.
var ErrNonStartingPacketAndNoPrevious = errors.New(
	"received a non-starting fragment without any previous starting fragment")

// Decoder is a RTP/KLV decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc6597
type Decoder struct {
	// buffer for accumulating KLV unit data across multiple packets
	buffer []byte
	// expected total size of the current KLV unit being assembled
	expectedSize int
	// timestamp of the current KLV unit being assembled
	currentTimestamp uint32
	// whether we're currently assembling a KLV unit
	assembling bool
	// sequence number of the last processed packet
	lastSeqNum uint16
	// whether we've received the first packet
	firstPacketReceived bool
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	d.reset()
	return nil
}

// reset clears the decoder state.
func (d *Decoder) reset() {
	d.buffer = d.buffer[:0]
	d.expectedSize = 0
	d.currentTimestamp = 0
	d.assembling = false
	d.firstPacketReceived = false
}

// parseKLVLength parses the KLV length field according to SMPTE ST 336.
// Returns the length value and the number of bytes consumed for the length field.
func parseKLVLength(data []byte, offset int) (length int, lengthSize int, err error) {
	if offset >= len(data) {
		return 0, 0, fmt.Errorf("insufficient data for length field")
	}

	firstByte := data[offset]
	
	// Short form: if bit 7 is 0, the length is in the lower 7 bits
	if (firstByte & 0x80) == 0 {
		return int(firstByte & 0x7f), 1, nil
	}

	// Long form: bit 7 is 1, lower 7 bits indicate number of subsequent length bytes
	lengthBytes := int(firstByte & 0x7f)
	if lengthBytes == 0 {
		return 0, 0, fmt.Errorf("indefinite length not supported")
	}
	if lengthBytes > 8 {
		return 0, 0, fmt.Errorf("length field too long: %d bytes", lengthBytes)
	}

	totalLengthSize := 1 + lengthBytes
	if offset+totalLengthSize > len(data) {
		return 0, 0, fmt.Errorf("insufficient data for length field")
	}

	// Parse the length value from the subsequent bytes
	var lengthValue int
	for i := 0; i < lengthBytes; i++ {
		lengthValue = (lengthValue << 8) | int(data[offset+1+i])
	}

	return lengthValue, totalLengthSize, nil
}

// isKLVStart checks if the payload starts with a KLV Universal Label Key.
// KLV Universal Label Keys start with the 4-byte prefix: 0x060e2b34
func isKLVStart(payload []byte) bool {
	if len(payload) < 4 {
		return false
	}
	return payload[0] == 0x06 && payload[1] == 0x0e && payload[2] == 0x2b && payload[3] == 0x34
}

// Decode decodes a KLV unit from RTP packets.
// It returns the complete KLV unit when all packets have been received,
// or ErrMorePacketsNeeded if more packets are needed.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, error) {
	payload := pkt.Payload
	marker := pkt.Marker
	timestamp := pkt.Timestamp
	seqNum := pkt.SequenceNumber

	// Check for sequence number gaps (packet loss)
	if d.firstPacketReceived {
		expectedSeq := d.lastSeqNum + 1
		if seqNum != expectedSeq {
			// Packet loss detected, reset state
			d.reset()
			return nil, fmt.Errorf("packet loss detected: expected seq %d, got %d", expectedSeq, seqNum)
		}
	}
	d.lastSeqNum = seqNum
	d.firstPacketReceived = true

	// If we're not currently assembling and this packet doesn't start a new KLV unit
	if !d.assembling {
		// Check if this looks like the start of a KLV unit
		if !isKLVStart(payload) {
			return nil, ErrNonStartingPacketAndNoPrevious
		}

		// This is the start of a new KLV unit
		d.currentTimestamp = timestamp
		d.assembling = true
		d.buffer = append(d.buffer[:0], payload...)

		// Try to determine the expected size if we have enough data
		if len(payload) >= 17 { // 16 bytes for Universal Label Key + at least 1 byte for length
			valueLength, lengthSize, err := parseKLVLength(payload, 16)
			if err == nil {
				d.expectedSize = 16 + lengthSize + valueLength
			}
		}
	} else {
		// We're assembling a KLV unit
		if timestamp != d.currentTimestamp {
			// Timestamp changed, this is a new KLV unit
			// The previous unit was incomplete
			d.reset()
			return nil, fmt.Errorf("incomplete KLV unit: timestamp changed from %d to %d", d.currentTimestamp, timestamp)
		}

		// Append this packet's payload to the buffer
		d.buffer = append(d.buffer, payload...)
	}

	// Check if we have a complete KLV unit
	if marker {
		// Marker bit indicates this is the last packet of the KLV unit
		result := make([]byte, len(d.buffer))
		copy(result, d.buffer)
		d.reset()
		return result, nil
	}

	// If we know the expected size and have reached it, return the complete unit
	if d.expectedSize > 0 && len(d.buffer) >= d.expectedSize {
		result := make([]byte, d.expectedSize)
		copy(result, d.buffer[:d.expectedSize])
		d.reset()
		return result, nil
	}

	// Need more packets
	return nil, ErrMorePacketsNeeded
}
