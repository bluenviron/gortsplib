package mikey

import "fmt"

// PayloadKEMACEncrAlg is a encryption algorithm.
type PayloadKEMACEncrAlg uint8

// RFC3830, Table 6.2.a
const (
	PayloadKEMACEncrAlgNULL PayloadKEMACEncrAlg = 0
)

// PayloadKEMACMacAlg is a authentication algorithm.
type PayloadKEMACMacAlg uint8

// RFC3830, Table 6.2.b
const (
	PayloadKEMACMacAlgNULL PayloadKEMACMacAlg = 0
)

// PayloadKEMAC is a Key data transport payload.
type PayloadKEMAC struct {
	EncrAlg     PayloadKEMACEncrAlg
	SubPayloads []*SubPayloadKeyData
	MacAlg      PayloadKEMACMacAlg
}

func (p *PayloadKEMAC) unmarshal(buf []byte) (int, error) {
	if len(buf) < 4 {
		return 0, fmt.Errorf("buffer too short")
	}

	n := 1
	p.EncrAlg = PayloadKEMACEncrAlg(buf[n])
	n++

	if p.EncrAlg != PayloadKEMACEncrAlgNULL {
		return 0, fmt.Errorf("unsupported encr alg: %v", p.EncrAlg)
	}

	encrDataLen := int(uint16(buf[n])<<8 | uint16(buf[n+1]))
	n += 2

	if len(buf[n:]) < (encrDataLen + 1) {
		return 0, fmt.Errorf("buffer too short")
	}

	encrData := buf[n : n+encrDataLen]
	n += encrDataLen

	sn := 0

	for {
		sp := &SubPayloadKeyData{}
		spLen, err := sp.unmarshal(encrData[sn:])
		if err != nil {
			return 0, err
		}

		nextPayloadType := payloadType(encrData[sn])
		sn += spLen
		p.SubPayloads = append(p.SubPayloads, sp)

		if nextPayloadType == 0 {
			break
		}
		if nextPayloadType != payloadTypeKeyData {
			return 0, fmt.Errorf("unsupported payload type: %v", nextPayloadType)
		}
	}

	if sn != len(encrData) {
		return 0, fmt.Errorf("detected unread bytes")
	}

	p.MacAlg = PayloadKEMACMacAlg(buf[n])
	n++

	if p.MacAlg != PayloadKEMACMacAlgNULL {
		return 0, fmt.Errorf("unsupported mac alg: %v", p.MacAlg)
	}

	return n, nil
}

func (*PayloadKEMAC) typ() payloadType {
	return payloadTypeKEMAC
}

func (p *PayloadKEMAC) marshalSize() int {
	n := 5
	for _, sp := range p.SubPayloads {
		n += sp.marshalSize()
	}
	return n
}

func (p *PayloadKEMAC) marshalTo(buf []byte) (int, error) {
	buf[1] = byte(p.EncrAlg)

	encrDataLen := 0
	for _, sp := range p.SubPayloads {
		encrDataLen += sp.marshalSize()
	}

	buf[2] = byte(encrDataLen >> 8)
	buf[3] = byte(encrDataLen)
	n := 4

	for i, sp := range p.SubPayloads {
		var nextPayloadType payloadType
		if i != len(p.SubPayloads)-1 {
			nextPayloadType = payloadTypeKeyData
		} else {
			nextPayloadType = 0
		}

		buf[n] = byte(nextPayloadType)

		n2, err := sp.marshalTo(buf[n:])
		if err != nil {
			return 0, err
		}
		n += n2
	}

	buf[n] = byte(p.MacAlg)
	n++

	return n, nil
}
