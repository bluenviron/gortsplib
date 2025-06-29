package mikey

import "fmt"

// PayloadT is a timestamp payload.
type PayloadT struct {
	TSType  uint8
	TSValue uint64
}

func (p *PayloadT) unmarshal(buf []byte) (int, error) {
	if len(buf) < 10 {
		return 0, fmt.Errorf("buffer too short")
	}

	n := 1
	p.TSType = buf[n]
	n++

	if p.TSType != 0 {
		return 0, fmt.Errorf("unsupported TSType: %v", p.TSType)
	}

	p.TSValue = uint64(buf[n])<<56 |
		uint64(buf[n+1])<<48 |
		uint64(buf[n+2])<<40 |
		uint64(buf[n+3])<<32 |
		uint64(buf[n+4])<<24 |
		uint64(buf[n+5])<<16 |
		uint64(buf[n+6])<<8 |
		uint64(buf[n+7])
	n += 8

	return n, nil
}

func (*PayloadT) typ() payloadType {
	return payloadTypeT
}

func (p *PayloadT) marshalSize() int {
	return 10
}

func (p *PayloadT) marshalTo(buf []byte) (int, error) {
	buf[1] = p.TSType
	buf[2] = byte(p.TSValue >> 56)
	buf[3] = byte(p.TSValue >> 48)
	buf[4] = byte(p.TSValue >> 40)
	buf[5] = byte(p.TSValue >> 32)
	buf[6] = byte(p.TSValue >> 24)
	buf[7] = byte(p.TSValue >> 16)
	buf[8] = byte(p.TSValue >> 8)
	buf[9] = byte(p.TSValue)
	return 10, nil
}
