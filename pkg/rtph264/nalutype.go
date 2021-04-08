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

// String implements fmt.Stringer.
func (nt naluType) String() string {
	switch nt {
	case naluTypeNonIDR:
		return "NonIDR"
	case naluTypeDataPartitionA:
		return "DataPartitionA"
	case naluTypeDataPartitionB:
		return "DataPartitionB"
	case naluTypeDataPartitionC:
		return "DataPartitionC"
	case naluTypeIDR:
		return "IDR"
	case naluTypeSEI:
		return "SEI"
	case naluTypeSPS:
		return "SPS"
	case naluTypePPS:
		return "PPS"
	case naluTypeAccessUnitDelimiter:
		return "AccessUnitDelimiter"
	case naluTypeEndOfSequence:
		return "EndOfSequence"
	case naluTypeEndOfStream:
		return "EndOfStream"
	case naluTypeFillerData:
		return "FillerData"
	case naluTypeSPSExtension:
		return "SPSExtension"
	case naluTypePrefix:
		return "Prefix"
	case naluTypeSubsetSPS:
		return "SubsetSPS"
	case naluTypeReserved16:
		return "Reserved16"
	case naluTypeReserved17:
		return "Reserved17"
	case naluTypeReserved18:
		return "Reserved18"
	case naluTypeSliceLayerWithoutPartitioning:
		return "SliceLayerWithoutPartitioning"
	case naluTypeSliceExtension:
		return "SliceExtension"
	case naluTypeSliceExtensionDepth:
		return "SliceExtensionDepth"
	case naluTypeReserved22:
		return "Reserved22"
	case naluTypeReserved23:
		return "Reserved23"
	case naluTypeSTAPA:
		return "STAPA"
	case naluTypeSTAPB:
		return "STAPB"
	case naluTypeMTAP16:
		return "MTAP16"
	case naluTypeMTAP24:
		return "MTAP24"
	case naluTypeFUA:
		return "FUA"
	case naluTypeFUB:
		return "FUB"
	}
	return "unknown"
}
