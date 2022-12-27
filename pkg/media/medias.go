package media

import (
	"fmt"

	"github.com/google/uuid"
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

// SetControls sets the control attribute of all medias in the list.
func (ms Medias) SetControls() {
	for _, media := range ms {
		media.Control = "mediaUUID=" + uuid.New().String()
	}
}

// FindFormat finds a certain format among all the formats in all the medias.
// If the format is found, it is inserted into forma, and its media is returned.
func (ms Medias) FindFormat(forma interface{}) *Media {
	for _, media := range ms {
		ok := media.FindFormat(forma)
		if ok {
			return media
		}
	}
	return nil
}
