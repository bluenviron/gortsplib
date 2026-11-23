package gortsplib

import (
	"github.com/bluenviron/gortsplib/v5/pkg/description"
)

func ntp64ExtensionID(medi *description.Media) uint8 {
	for _, ext := range medi.HeaderExtensions {
		if ext.URI == description.NTP64ExtensionURI {
			return ext.ID
		}
	}
	return 0
}
