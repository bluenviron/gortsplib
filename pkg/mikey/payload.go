package mikey

type payloadType uint8

// RFC3830, table 6.1.b
const (
	payloadTypeKEMAC   payloadType = 1
	payloadTypeT       payloadType = 5
	payloadTypeSP      payloadType = 10
	payloadTypeRAND    payloadType = 11
	payloadTypeKeyData payloadType = 20
)

// Payload is a MIKEY payload.
type Payload interface {
	unmarshal(buf []byte) (int, error)
	typ() payloadType
	marshalSize() int
	marshalTo(buf []byte) (int, error)
}
