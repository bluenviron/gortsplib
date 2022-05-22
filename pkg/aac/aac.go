// Package aac contains utilities to work with the AAC codec.
package aac

const (
	// MaxAccessUnitSize is the maximum size of an Access Unit (AU).
	MaxAccessUnitSize = 5 * 1024

	// SamplesPerAccessUnit is the number of samples contained by a single AAC AU.
	SamplesPerAccessUnit = 1024
)
