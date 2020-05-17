package gortsplib

import (
	"fmt"
	"strconv"

	"gortc.io/sdp"
)

// SDPParse parses a SDP document.
func SDPParse(in []byte) (*sdp.Message, error) {
	s, err := sdp.DecodeSession(in, nil)
	if err != nil {
		return nil, err
	}

	m := &sdp.Message{}
	d := sdp.NewDecoder(s)
	err = d.Decode(m)
	if err != nil {
		// allow empty Origins
		if err.Error() != "failed to decode message: DecodeError in section s: origin address not set" {
			return nil, err
		}
	}

	if len(m.Medias) == 0 {
		return nil, fmt.Errorf("no tracks defined")
	}

	return m, nil
}

// SDPFilter removes everything from a SDP document, except the bare minimum.
func SDPFilter(msgIn *sdp.Message, byteIn []byte) (*sdp.Message, []byte) {
	msgOut := &sdp.Message{}

	msgOut.Name = "Stream"
	msgOut.Origin = sdp.Origin{
		Username:    "-",
		NetworkType: "IN",
		AddressType: "IP4",
		Address:     "127.0.0.1",
	}

	for i, m := range msgIn.Medias {
		var attributes []sdp.Attribute
		for _, attr := range m.Attributes {
			if attr.Key == "rtpmap" || attr.Key == "fmtp" {
				attributes = append(attributes, attr)
			}
		}

		// control attribute is mandatory, and is the path that is appended
		// to the stream path in SETUP
		attributes = append(attributes, sdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})

		msgOut.Medias = append(msgOut.Medias, sdp.Media{
			Bandwidths: m.Bandwidths,
			Description: sdp.MediaDescription{
				Type:     m.Description.Type,
				Protocol: "RTP/AVP", // override protocol
				Formats:  m.Description.Formats,
			},
			Attributes: attributes,
		})
	}

	sdps := sdp.Session{}
	sdps = msgOut.Append(sdps)
	byteOut := sdps.AppendTo(nil)

	return msgOut, byteOut
}
