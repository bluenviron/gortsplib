package mikey

import "fmt"

// PayloadRAND is a payload with random data.
type PayloadRAND struct {
	Data []byte
}

func (p *PayloadRAND) unmarshal(buf []byte) (int, error) {
	if len(buf) < 2 {
		return 0, fmt.Errorf("buffer too short")
	}

	n := 1
	dataLen := int(buf[n])
	n++

	if dataLen < 16 {
		return 0, fmt.Errorf("invalid data len: %v", dataLen)
	}

	if len(buf[n:]) < dataLen {
		return 0, fmt.Errorf("buffer too short")
	}

	p.Data = buf[n : n+dataLen]
	n += dataLen

	return n, nil
}

func (*PayloadRAND) typ() payloadType {
	return payloadTypeRAND
}

func (p *PayloadRAND) marshalSize() int {
	return 2 + len(p.Data)
}

func (p *PayloadRAND) marshalTo(buf []byte) (int, error) {
	buf[1] = uint8(len(p.Data))
	n := 2
	n += copy(buf[2:], p.Data)
	return n, nil
}
