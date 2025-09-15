package headers

import (
	"fmt"
	"strings"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

// Transports is a Transport header with multiple transports.
type Transports []Transport

// Unmarshal decodes a Transport header.
func (ts *Transports) Unmarshal(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	v0 := v[0]
	transports := strings.Split(v0, ",") // , separated per RFC2326 section 12.39
	*ts = make([]Transport, len(transports))

	for i, transport := range transports {
		var tr Transport
		err := tr.Unmarshal(base.HeaderValue{strings.TrimLeft(transport, " ")})
		if err != nil {
			return err
		}
		(*ts)[i] = tr
	}

	return nil
}

// Marshal encodes a Transport header.
func (ts Transports) Marshal() base.HeaderValue {
	vals := make([]string, len(ts))

	for i, th := range ts {
		vals[i] = th.Marshal()[0]
	}

	return base.HeaderValue{strings.Join(vals, ",")}
}
