package mikey

import "fmt"

// PayloadSPProtType is a security protocol.
type PayloadSPProtType uint8

// RFC3830, Table 6.2.a
const (
	PayloadSPProtTypeSRTP PayloadSPProtType = 0
)

// PayloadSPPolicyParamType is a policy param type.
type PayloadSPPolicyParamType uint8

// RFC3830, Table 6.10.1.a
const (
	PayloadSPPolicyParamTypeEncrAlg           PayloadSPPolicyParamType = 0
	PayloadSPPolicyParamTypeSessionEncrKeyLen PayloadSPPolicyParamType = 1
	PayloadSPPolicyParamTypeAuthAlg           PayloadSPPolicyParamType = 2
	PayloadSPPolicyParamTypeSessionAuthKeyLen PayloadSPPolicyParamType = 3
	PayloadSPPolicyParamTypeSessionSaltKeyLen PayloadSPPolicyParamType = 4
	PayloadSPPolicyParamTypeSRTPPseudoRandFun PayloadSPPolicyParamType = 5
	PayloadSPPolicyParamTypeKeyDerRate        PayloadSPPolicyParamType = 6
	PayloadSPPolicyParamTypeSRTPEncrOffOn     PayloadSPPolicyParamType = 7
	PayloadSPPolicyParamTypeSRTCPEncrOffOn    PayloadSPPolicyParamType = 8
	PayloadSPPolicyParamTypeSenderFECOrder    PayloadSPPolicyParamType = 9
	PayloadSPPolicyParamTypeSRTPAuthOffOn     PayloadSPPolicyParamType = 10
	PayloadSPPolicyParamTypeAuthTagLen        PayloadSPPolicyParamType = 11
	PayloadSPPolicyParamTypeSRTPPrefixLen     PayloadSPPolicyParamType = 12
)

// PayloadSPPolicyParam is a policy param.
type PayloadSPPolicyParam struct {
	Type  PayloadSPPolicyParamType
	Value []byte
}

// PayloadSP is a security policy payload.
type PayloadSP struct {
	PolicyNo     uint8
	ProtType     PayloadSPProtType
	PolicyParams []PayloadSPPolicyParam
}

func (p *PayloadSP) unmarshal(buf []byte) (int, error) {
	if len(buf) < 5 {
		return 0, fmt.Errorf("buffer too short")
	}

	n := 1
	p.PolicyNo = buf[n]
	n++
	p.ProtType = PayloadSPProtType(buf[n])
	n++

	if p.ProtType != 0 {
		return 0, fmt.Errorf("unsupported prot type: %v", p.ProtType)
	}

	policyParamLength := uint16(buf[n])<<8 | uint16(buf[n+1])
	n += 2
	end := n + int(policyParamLength)

	for {
		if n > end {
			return 0, fmt.Errorf("policy param overflowed")
		}
		if n == end {
			break
		}
		if len(buf[n:]) < 2 {
			return 0, fmt.Errorf("buffer too short")
		}

		typ := PayloadSPPolicyParamType(buf[n])
		n++
		valueLen := int(buf[n])
		n++

		if len(buf[n:]) < valueLen {
			return 0, fmt.Errorf("buffer too short")
		}

		value := buf[n : n+valueLen]
		n += valueLen

		p.PolicyParams = append(p.PolicyParams, PayloadSPPolicyParam{
			Type:  typ,
			Value: value,
		})
	}

	return n, nil
}

func (*PayloadSP) typ() payloadType {
	return payloadTypeSP
}

func (p *PayloadSP) marshalSize() int {
	n := 5 + 2*len(p.PolicyParams)
	for _, pp := range p.PolicyParams {
		n += len(pp.Value)
	}
	return n
}

func (p *PayloadSP) marshalTo(buf []byte) (int, error) {
	buf[1] = p.PolicyNo
	buf[2] = byte(p.ProtType)

	policyParamLength := 0
	for _, pp := range p.PolicyParams {
		policyParamLength += 2 + len(pp.Value)
	}
	buf[3] = byte(policyParamLength >> 8)
	buf[4] = byte(policyParamLength)
	n := 5

	for _, pp := range p.PolicyParams {
		buf[n] = byte(pp.Type)
		buf[n+1] = uint8(len(pp.Value))
		n += 2
		n += copy(buf[n:], pp.Value)
	}

	return n, nil
}
