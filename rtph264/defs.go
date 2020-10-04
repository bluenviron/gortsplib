package rtph264

type naluType uint8

const (
	naluTypeFirstSingle naluType = 1
	naluTypeSPS         naluType = 7
	naluTypePPS         naluType = 8
	naluTypeLastSingle  naluType = 23
	naluTypeStapA       naluType = 24
	naluTypeStapB       naluType = 25
	naluTypeMtap16      naluType = 26
	naluTypeMtap24      naluType = 27
	naluTypeFuA         naluType = 28
	naluTypeFuB         naluType = 29
)
