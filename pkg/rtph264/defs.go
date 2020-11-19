package rtph264

// NALUType is the type of a NALU.
type NALUType uint8

// standard NALU types.
const (
	NALUTypeNonIDR                        NALUType = 1
	NALUTypeDataPartitionA                NALUType = 2
	NALUTypeDataPartitionB                NALUType = 3
	NALUTypeDataPartitionC                NALUType = 4
	NALUTypeIDR                           NALUType = 5
	NALUTypeSei                           NALUType = 6
	NALUTypeSPS                           NALUType = 7
	NALUTypePPS                           NALUType = 8
	NALUTypeAccessUnitDelimiter           NALUType = 9
	NALUTypeEndOfSequence                 NALUType = 10
	NALUTypeEndOfStream                   NALUType = 11
	NALUTypeFillerData                    NALUType = 12
	NALUTypeSPSExtension                  NALUType = 13
	NALUTypePrefix                        NALUType = 14
	NALUTypeSubsetSPS                     NALUType = 15
	NALUTypeReserved16                    NALUType = 16
	NALUTypeReserved17                    NALUType = 17
	NALUTypeReserved18                    NALUType = 18
	NALUTypeSliceLayerWithoutPartitioning NALUType = 19
	NALUTypeSliceExtension                NALUType = 20
	NALUTypeSliceExtensionDepth           NALUType = 21
	NALUTypeReserved22                    NALUType = 22
	NALUTypeReserved23                    NALUType = 23
	NALUTypeStapA                         NALUType = 24
	NALUTypeStapB                         NALUType = 25
	NALUTypeMtap16                        NALUType = 26
	NALUTypeMtap24                        NALUType = 27
	NALUTypeFuA                           NALUType = 28
	NALUTypeFuB                           NALUType = 29
)
