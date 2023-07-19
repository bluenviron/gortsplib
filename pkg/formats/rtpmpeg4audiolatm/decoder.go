package rtpmpeg4audiolatm

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

func joinFragments(fragments [][]byte, size int) []byte {
	ret := make([]byte, size)
	n := 0
	for _, p := range fragments {
		n += copy(ret[n:], p)
	}
	return ret
}

// Decoder is a RTP/MPEG-4 Audio decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416#section-7.3
type Decoder struct {
	// StreamMuxConfig.
	Config *mpeg4audio.StreamMuxConfig

	timeDecoder       *rtptime.Decoder
	fragments         [][]byte
	fragmentsSize     int
	fragmentsExpected int
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	if d.Config == nil || len(d.Config.Programs) != 1 || len(d.Config.Programs[0].Layers) != 1 {
		return fmt.Errorf("unsupported StreamMuxConfig")
	}

	d.timeDecoder = rtptime.NewDecoder(d.Config.Programs[0].Layers[0].AudioSpecificConfig.SampleRate)
	return nil
}

// Decode decodes an AU from a RTP packet.
// It returns the AU and its PTS.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	var au []byte
	buf := pkt.Payload

	if len(d.fragments) == 0 {
		pl, n, err := payloadLengthInfoDecode(buf)
		if err != nil {
			return nil, 0, err
		}

		buf = buf[n:]
		bl := len(buf)

		if pl <= bl {
			au = buf[:pl]
			// there could be other data, due to otherDataPresent. Ignore it.
		} else {
			if pl > mpeg4audio.MaxAccessUnitSize {
				d.fragments = d.fragments[:0] // discard pending fragments
				return nil, 0, fmt.Errorf("AU size (%d) is too big, maximum is %d", pl, mpeg4audio.MaxAccessUnitSize)
			}

			d.fragments = append(d.fragments, buf)
			d.fragmentsSize = pl
			d.fragmentsExpected = pl - bl
			return nil, 0, ErrMorePacketsNeeded
		}
	} else {
		bl := len(buf)

		if d.fragmentsExpected > bl {
			d.fragments = append(d.fragments, buf)
			d.fragmentsExpected -= bl
			return nil, 0, ErrMorePacketsNeeded
		}

		d.fragments = append(d.fragments, buf[:d.fragmentsExpected])
		// there could be other data, due to otherDataPresent. Ignore it.

		au = joinFragments(d.fragments, d.fragmentsSize)
		d.fragments = d.fragments[:0]
	}

	return au, d.timeDecoder.Decode(pkt.Timestamp), nil
}
