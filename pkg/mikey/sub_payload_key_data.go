package mikey

import "fmt"

// SubPayloadKeyDataType is a key type.
type SubPayloadKeyDataType uint8

// RFC3830, table 6.13.a
const (
	SubPayloadKeyDataTypeTEK SubPayloadKeyDataType = 2
)

// SubPayloadKeyDataKV is a KV (key validity) value.
type SubPayloadKeyDataKV uint8

// RFC3830, table 6.13.b
const (
	SubPayloadKeyDataKVNull SubPayloadKeyDataKV = 0
	SubPayloadKeyDataKVSPI  SubPayloadKeyDataKV = 1
)

// SubPayloadKeyData is a key data sub-payload.
type SubPayloadKeyData struct {
	Type    SubPayloadKeyDataType
	KV      SubPayloadKeyDataKV
	KeyData []byte
	SPI     []byte
}

func (p *SubPayloadKeyData) unmarshal(buf []byte) (int, error) {
	if len(buf) < 4 {
		return 0, fmt.Errorf("buffer too short")
	}

	n := 1
	p.Type = SubPayloadKeyDataType(buf[n] >> 4)
	p.KV = SubPayloadKeyDataKV(buf[n] & 0b1111)
	n++

	if p.Type != SubPayloadKeyDataTypeTEK {
		return 0, fmt.Errorf("unsupported key type: %v", p.Type)
	}

	if p.KV != SubPayloadKeyDataKVNull && p.KV != SubPayloadKeyDataKVSPI {
		return 0, fmt.Errorf("unsupported KV: %v", p.KV)
	}

	keyDataLen := int(uint16(buf[n])<<8 | uint16(buf[n+1]))
	n += 2

	if len(buf[n:]) < keyDataLen {
		return 0, fmt.Errorf("buffer too short")
	}

	p.KeyData = buf[n : n+keyDataLen]
	n += keyDataLen

	if p.KV == SubPayloadKeyDataKVSPI {
		if len(buf[n:]) < 1 {
			return 0, fmt.Errorf("buffer too short")
		}

		spiLen := int(buf[n])
		n++

		if len(buf[n:]) < spiLen {
			return 0, fmt.Errorf("buffer too short")
		}

		p.SPI = buf[n : n+spiLen]
		n += spiLen
	}

	return n, nil
}

func (p SubPayloadKeyData) marshalSize() int {
	n := 4 + len(p.KeyData)
	if p.KV == SubPayloadKeyDataKVSPI {
		n += 1 + len(p.SPI)
	}
	return n
}

func (p SubPayloadKeyData) marshalTo(buf []byte) (int, error) {
	buf[1] = byte(p.Type)<<4 | byte(p.KV)

	keyDataLen := len(p.KeyData)
	buf[2] = byte(keyDataLen >> 8)
	buf[3] = byte(keyDataLen)
	n := 4

	n += copy(buf[n:], p.KeyData)

	if p.KV == SubPayloadKeyDataKVSPI {
		buf[n] = uint8(len(p.SPI))
		n++
		n += copy(buf[n:], p.SPI)
	}

	return n, nil
}
