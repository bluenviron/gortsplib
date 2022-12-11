package media

import (
	"fmt"
	"reflect"
	"strconv"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/v2/pkg/sdp"
)

// Medias is a list of media streams.
type Medias []*Media

// Unmarshal decodes medias from the SDP format.
func (ms *Medias) Unmarshal(mds []*psdp.MediaDescription) error {
	*ms = make(Medias, len(mds))

	for i, md := range mds {
		var m Media
		err := m.unmarshal(md)
		if err != nil {
			return fmt.Errorf("media %d is invalid: %v", i+1, err)
		}
		(*ms)[i] = &m
	}

	return nil
}

// Marshal encodes the medias in SDP format.
func (ms Medias) Marshal(multicast bool) *sdp.SessionDescription {
	var address string
	if multicast {
		address = "224.1.0.0"
	} else {
		address = "0.0.0.0"
	}

	sout := &sdp.SessionDescription{
		SessionName: psdp.SessionName("Stream"),
		Origin: psdp.Origin{
			Username:       "-",
			NetworkType:    "IN",
			AddressType:    "IP4",
			UnicastAddress: "127.0.0.1",
		},
		// required by Darwin Streaming Server
		ConnectionInformation: &psdp.ConnectionInformation{
			NetworkType: "IN",
			AddressType: "IP4",
			Address:     &psdp.Address{Address: address},
		},
		TimeDescriptions: []psdp.TimeDescription{
			{Timing: psdp.Timing{StartTime: 0, StopTime: 0}},
		},
		MediaDescriptions: make([]*psdp.MediaDescription, len(ms)),
	}

	for i, media := range ms {
		sout.MediaDescriptions[i] = media.Marshal()
	}

	return sout
}

// Clone clones the media list.
func (ms Medias) Clone() Medias {
	ret := make(Medias, len(ms))
	for i, media := range ms {
		ret[i] = media.Clone()
	}
	return ret
}

// CloneAndSetControls clones the media list and sets the control attribute
// of all medias in the list.
func (ms Medias) CloneAndSetControls() Medias {
	ret := ms.Clone()
	ret.SetControls()
	return ret
}

// SetControls sets the control attribute of all medias in the list.
func (ms Medias) SetControls() {
	for i, media := range ms {
		media.Control = "mediaID=" + strconv.FormatInt(int64(i), 10)
	}
}

// FindFormat finds a certain format among all the formats in all the medias.
// If the format is found, it is inserted into trak, and format media is returned.
func (ms *Medias) FindFormat(trak interface{}) *Media {
	for _, media := range *ms {
		for _, trakk := range media.Formats {
			if reflect.TypeOf(trakk) == reflect.TypeOf(trak).Elem() {
				reflect.ValueOf(trak).Elem().Set(reflect.ValueOf(trakk))
				return media
			}
		}
	}
	return nil
}
