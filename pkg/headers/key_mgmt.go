package headers

import (
	"encoding/base64"
	"fmt"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/mikey"
)

// KeyMgmt is a KeyMgmt header.
type KeyMgmt struct {
	URL          string
	MikeyMessage *mikey.Message
}

// Unmarshal decodes a KeyMgmt header.
func (h *KeyMgmt) Unmarshal(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	kvs, err := keyValParse(v[0], ';')
	if err != nil {
		return err
	}

	protocolProvided := false
	uriProvided := false

	for k, v := range kvs {
		switch k {
		case "prot":
			if v != "mikey" {
				return fmt.Errorf("unsupported protocol: %v", v)
			}
			protocolProvided = true

		case "uri":
			h.URL = v
			uriProvided = true

		case "data":
			var byts []byte
			byts, err = base64.StdEncoding.DecodeString(v)
			if err != nil {
				return fmt.Errorf("invalid data: %w", err)
			}

			h.MikeyMessage = &mikey.Message{}
			err = h.MikeyMessage.Unmarshal(byts)
			if err != nil {
				return fmt.Errorf("invalid data: %w", err)
			}
		}
	}

	if !protocolProvided {
		return fmt.Errorf("protocol not provided")
	}

	if !uriProvided {
		return fmt.Errorf("URI not provided")
	}

	if h.MikeyMessage == nil {
		return fmt.Errorf("mikey message not provided")
	}

	return nil
}

// Marshal encodes a KeyMgmt header.
func (h KeyMgmt) Marshal() (base.HeaderValue, error) {
	buf, err := h.MikeyMessage.Marshal()
	if err != nil {
		return nil, err
	}

	encData := base64.StdEncoding.EncodeToString(buf)

	return base.HeaderValue{`prot=mikey;uri="` + h.URL + `";data="` + encData + `"`}, nil
}
