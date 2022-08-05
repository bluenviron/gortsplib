package mpeg4audio

// ObjectType is a MPEG-4 Audio object type.
type ObjectType int

// supported types.
const (
	ObjectTypeAACLC ObjectType = 2
	ObjectTypeSBR   ObjectType = 5
)
