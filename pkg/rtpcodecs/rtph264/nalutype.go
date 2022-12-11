package rtph264

import (
	"fmt"
	"strings"

	"github.com/aler9/gortsplib/v2/pkg/h264"
)

type naluType h264.NALUType

// additional NALU types for RTP/H264.
const (
	naluTypeSTAPA  naluType = 24
	naluTypeSTAPB  naluType = 25
	naluTypeMTAP16 naluType = 26
	naluTypeMTAP24 naluType = 27
	naluTypeFUA    naluType = 28
	naluTypeFUB    naluType = 29
)

var naluLabels = map[naluType]string{
	naluTypeSTAPA:  "STAP-A",
	naluTypeSTAPB:  "STAP-B",
	naluTypeMTAP16: "MTAP-16",
	naluTypeMTAP24: "MTAP-24",
	naluTypeFUA:    "FU-A",
	naluTypeFUB:    "FU-B",
}

// String implements fmt.Stringer.
func (nt naluType) String() string {
	p := h264.NALUType(nt).String()
	if !strings.HasPrefix(p, "unknown") {
		return p
	}

	if l, ok := naluLabels[nt]; ok {
		return l
	}

	return fmt.Sprintf("unknown (%d)", nt)
}
