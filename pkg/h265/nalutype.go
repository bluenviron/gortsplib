package h265

import (
	"fmt"
)

// NALUType is the type of a NALU.
type NALUType uint8

// NALU types.
const (
	NALUTypeTrailingN      NALUType = 0
	NALUTypeTrailingR      NALUType = 1
	NALUTypeTSAN           NALUType = 2
	NALUTypeTSAR           NALUType = 3
	NALUTypeSTSAN          NALUType = 4
	NALUTypeSTSAR          NALUType = 5
	NALUTypeRADLN          NALUType = 6
	NALUTypeRADLR          NALUType = 7
	NALUTypeRASLN          NALUType = 8
	NALUTypeRASLR          NALUType = 9
	NALUTypeReservedVLCN10 NALUType = 10
	NALUTypeReservedVLCN12 NALUType = 12
	NALUTypeReservedVLCN14 NALUType = 14
	NALUTypeReservedVLCR11 NALUType = 11
	NALUTypeReservedVLCR13 NALUType = 13
	NALUTypeReservedVLCR15 NALUType = 15
	NALUTypeBLAWLP         NALUType = 16
	NALUTypeBLAWRADL       NALUType = 17
	NALUTypeBLANLP         NALUType = 18
	NALUTypeIDRWRADL       NALUType = 19
	NALUTypeIDRNLP         NALUType = 20
	NALUTypeCRANUT         NALUType = 21
	NALUTypeVPS            NALUType = 32
	NALUTypeSPS            NALUType = 33
	NALUTypePPS            NALUType = 34
	NALUTypeAUD            NALUType = 35
	NALUTypeEOS            NALUType = 36
	NALUTypeEOB            NALUType = 37
	NALUTypeFD             NALUType = 38
	NALUTypePrefixSEINUT   NALUType = 39
	NALUTypeSuffixSEINUT   NALUType = 40

	// additional NALU types for RTP/H265
	NALUTypeAggregationUnit   NALUType = 48
	NALUTypeFragmentationUnit NALUType = 49
	NALUTypePACI              NALUType = 50
)

var naluTypeLabels = map[NALUType]string{
	NALUTypeTrailingN:      "TrailingN",
	NALUTypeTrailingR:      "TrailingR",
	NALUTypeTSAN:           "TSAN",
	NALUTypeTSAR:           "TSAR",
	NALUTypeSTSAN:          "STSAN",
	NALUTypeSTSAR:          "STSAR",
	NALUTypeRADLN:          "RADLN",
	NALUTypeRADLR:          "RADLR",
	NALUTypeRASLN:          "RASLN",
	NALUTypeRASLR:          "RASLR",
	NALUTypeReservedVLCN10: "ReservedVLCN10",
	NALUTypeReservedVLCN12: "ReservedVLCN12",
	NALUTypeReservedVLCN14: "ReservedVLCN14",
	NALUTypeReservedVLCR11: "ReservedVLCR11",
	NALUTypeReservedVLCR13: "ReservedVLCR13",
	NALUTypeReservedVLCR15: "ReservedVLCR15",
	NALUTypeBLAWLP:         "BLAWLP",
	NALUTypeBLAWRADL:       "BLAWRADL",
	NALUTypeBLANLP:         "BLANLP",
	NALUTypeIDRWRADL:       "IDRWRADL",
	NALUTypeIDRNLP:         "IDRNLP",
	NALUTypeCRANUT:         "CRANUT",
	NALUTypeVPS:            "VPS",
	NALUTypeSPS:            "SPS",
	NALUTypePPS:            "PPS",
	NALUTypeAUD:            "AUD",
	NALUTypeEOS:            "EOS",
	NALUTypeEOB:            "EOB",
	NALUTypeFD:             "FD",
	NALUTypePrefixSEINUT:   "PrefixSEINUT",
	NALUTypeSuffixSEINUT:   "SuffixSEINUT",

	// additional NALU types for RTP/H265
	NALUTypeAggregationUnit:   "AggregationUnit",
	NALUTypeFragmentationUnit: "FragmentationUnit",
	NALUTypePACI:              "PACI",
}

// String implements fmt.Stringer.
func (nt NALUType) String() string {
	if l, ok := naluTypeLabels[nt]; ok {
		return l
	}
	return fmt.Sprintf("unknown (%d)", nt)
}
