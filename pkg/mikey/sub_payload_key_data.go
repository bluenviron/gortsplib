package mikey

import "fmt"

// SubPayloadKeyDataKeyType is a data key type.
type SubPayloadKeyDataKeyType uint8

// RFC3830, table 6.13.a
const (
	SubPayloadKeyDataKeyTypeTEK SubPayloadKeyDataKeyType = 2
)

// SubPayloadKeyData is a key data sub-payload.
type SubPayloadKeyData struct {
	Type    SubPayloadKeyDataKeyType
	KV      uint8
	KeyData []byte
}

func (p *SubPayloadKeyData) unmarshal(buf []byte) (int, error) {
	if len(buf) < 4 {
		return 0, fmt.Errorf("buffer too short")
	}

	n := 1
	p.Type = SubPayloadKeyDataKeyType(buf[n] >> 4)
	p.KV = buf[n] & 0b00001111
	n++

	if p.Type != SubPayloadKeyDataKeyTypeTEK {
		return 0, fmt.Errorf("unsupported key type: %v", p.Type)
	}

	if p.KV != 0 {
		return 0, fmt.Errorf("unsupported KV: %v", p.KV)
	}

	keyDataLen := int(uint16(buf[n])<<8 | uint16(buf[n+1]))
	n += 2

	if len(buf[n:]) < keyDataLen {
		return 0, fmt.Errorf("buffer too short")
	}

	p.KeyData = buf[n : n+keyDataLen]
	n += keyDataLen

	return n, nil
}

func (p *SubPayloadKeyData) marshalSize() int {
	return 4 + len(p.KeyData)
}

func (p *SubPayloadKeyData) marshalTo(buf []byte) (int, error) {
	buf[1] = byte(p.Type)<<4 | p.KV

	keyDataLen := len(p.KeyData)
	buf[2] = byte(keyDataLen >> 8)
	buf[3] = byte(keyDataLen)
	n := 4

	n += copy(buf[n:], p.KeyData)

	return n, nil
}
