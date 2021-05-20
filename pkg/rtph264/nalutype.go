package rtph264

// naluType is the type of a NALU.
type naluType uint8

// NALU types, augmented for RTP.
const (
	naluTypeNonIDR                        naluType = 1
	naluTypeDataPartitionA                naluType = 2
	naluTypeDataPartitionB                naluType = 3
	naluTypeDataPartitionC                naluType = 4
	naluTypeIDR                           naluType = 5
	naluTypeSEI                           naluType = 6
	naluTypeSPS                           naluType = 7
	naluTypePPS                           naluType = 8
	naluTypeAccessUnitDelimiter           naluType = 9
	naluTypeEndOfSequence                 naluType = 10
	naluTypeEndOfStream                   naluType = 11
	naluTypeFillerData                    naluType = 12
	naluTypeSPSExtension                  naluType = 13
	naluTypePrefix                        naluType = 14
	naluTypeSubsetSPS                     naluType = 15
	naluTypeReserved16                    naluType = 16
	naluTypeReserved17                    naluType = 17
	naluTypeReserved18                    naluType = 18
	naluTypeSliceLayerWithoutPartitioning naluType = 19
	naluTypeSliceExtension                naluType = 20
	naluTypeSliceExtensionDepth           naluType = 21
	naluTypeReserved22                    naluType = 22
	naluTypeReserved23                    naluType = 23
	naluTypeSTAPA                         naluType = 24
	naluTypeSTAPB                         naluType = 25
	naluTypeMTAP16                        naluType = 26
	naluTypeMTAP24                        naluType = 27
	naluTypeFUA                           naluType = 28
	naluTypeFUB                           naluType = 29
)

var naluLabels = map[naluType]string{
	naluTypeNonIDR:                        "NonIDR",
	naluTypeDataPartitionA:                "DataPartitionA",
	naluTypeDataPartitionB:                "DataPartitionB",
	naluTypeDataPartitionC:                "DataPartitionC",
	naluTypeIDR:                           "IDR",
	naluTypeSEI:                           "SEI",
	naluTypeSPS:                           "SPS",
	naluTypePPS:                           "PPS",
	naluTypeAccessUnitDelimiter:           "AccessUnitDelimiter",
	naluTypeEndOfSequence:                 "EndOfSequence",
	naluTypeEndOfStream:                   "EndOfStream",
	naluTypeFillerData:                    "FillerData",
	naluTypeSPSExtension:                  "SPSExtension",
	naluTypePrefix:                        "Prefix",
	naluTypeSubsetSPS:                     "SubsetSPS",
	naluTypeReserved16:                    "Reserved16",
	naluTypeReserved17:                    "Reserved17",
	naluTypeReserved18:                    "Reserved18",
	naluTypeSliceLayerWithoutPartitioning: "SliceLayerWithoutPartitioning",
	naluTypeSliceExtension:                "SliceExtension",
	naluTypeSliceExtensionDepth:           "SliceExtensionDepth",
	naluTypeReserved22:                    "Reserved22",
	naluTypeReserved23:                    "Reserved23",
	naluTypeSTAPA:                         "STAPA",
	naluTypeSTAPB:                         "STAPB",
	naluTypeMTAP16:                        "MTAP16",
	naluTypeMTAP24:                        "MTAP24",
	naluTypeFUA:                           "FUA",
	naluTypeFUB:                           "FUB",
}

// String implements fmt.Stringer.
func (nt naluType) String() string {
	if l, ok := naluLabels[nt]; ok {
		return l
	}
	return "unknown"
}
