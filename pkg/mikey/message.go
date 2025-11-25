// Package mikey contains functions to decode and encode MIKEY messages.
package mikey

import "fmt"

// Message is a MIKEY message.
type Message struct {
	Header   Header
	Payloads []Payload
}

// Unmarshal decodes a Message.
func (m *Message) Unmarshal(buf []byte) error {
	n, nextPayloadType, err := m.Header.unmarshal(buf)
	if err != nil {
		return err
	}

	for nextPayloadType != 0 {
		var payload Payload

		switch nextPayloadType {
		case payloadTypeKEMAC:
			payload = &PayloadKEMAC{}
		case payloadTypeT:
			payload = &PayloadT{}
		case payloadTypeSP:
			payload = &PayloadSP{}
		case payloadTypeRAND:
			payload = &PayloadRAND{}
		default:
			return fmt.Errorf("unsupported payload type: %d", nextPayloadType)
		}

		var payloadLen int
		payloadLen, err = payload.unmarshal(buf[n:])
		if err != nil {
			return fmt.Errorf("unable to parse payload %d: %w", nextPayloadType, err)
		}

		nextPayloadType = payloadType(buf[n])
		n += payloadLen
		m.Payloads = append(m.Payloads, payload)
	}

	// Some implementations (e.g., Milestone XProtect) add 1 byte of padding (0x00)
	// at the end. Even though this is not part of RFC 3830, allow it.
	if len(buf)-n > 1 && buf[n] != 0x00 {
		return fmt.Errorf("detected %d unparsed bytes", len(buf)-n)
	}

	return nil
}

func (m *Message) marshalSize() int {
	n := m.Header.marshalSize()
	for _, pl := range m.Payloads {
		n += pl.marshalSize()
	}
	return n
}

// Marshal encodes a Message.
func (m *Message) Marshal() ([]byte, error) {
	buf := make([]byte, m.marshalSize())

	var nextPayloadType payloadType
	if len(m.Payloads) != 0 {
		nextPayloadType = m.Payloads[0].typ()
	}

	n, err := m.Header.marshalTo(buf, nextPayloadType)
	if err != nil {
		return nil, err
	}

	for i, pl := range m.Payloads {
		if i != len(m.Payloads)-1 {
			nextPayloadType = m.Payloads[i+1].typ()
		} else {
			nextPayloadType = 0
		}

		buf[n] = byte(nextPayloadType)

		var n2 int
		n2, err = pl.marshalTo(buf[n:])
		if err != nil {
			return nil, err
		}
		n += n2
	}

	return buf, nil
}
