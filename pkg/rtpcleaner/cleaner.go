// Package rtpcleaner contains a cleaning utility.
package rtpcleaner

import (
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/aler9/gortsplib/pkg/rtph264"
)

// Output is the output of Clear().
type Output struct {
	Packet       *rtp.Packet
	PTSEqualsDTS bool
	H264NALUs    [][]byte
	H264PTS      time.Duration
}

// Cleaner is used to clean incoming RTP packets, in order to:
// - remove padding
// - re-encode them if they are bigger than maximum allowed
type Cleaner struct {
	isH264 bool
	isTCP  bool

	h264Decoder *rtph264.Decoder
}

// New allocates a Cleaner.
func New(isH264 bool, isTCP bool) *Cleaner {
	p := &Cleaner{
		isH264: isH264,
		isTCP:  isTCP,
	}

	if isH264 {
		p.h264Decoder = &rtph264.Decoder{}
		p.h264Decoder.Init()
	}

	return p
}

func (p *Cleaner) processH264(pkt *rtp.Packet) ([]*Output, error) {
	// decode
	nalus, pts, err := p.h264Decoder.DecodeUntilMarker(pkt)
	if err != nil {
		if err == rtph264.ErrNonStartingPacketAndNoPrevious ||
			err == rtph264.ErrMorePacketsNeeded { // hide standard errors
			err = nil
		}

		return []*Output{{
			Packet:       pkt,
			PTSEqualsDTS: false,
		}}, err
	}

	return []*Output{{
		Packet:       pkt,
		PTSEqualsDTS: h264.IDRPresent(nalus),
		H264NALUs:    nalus,
		H264PTS:      pts,
	}}, nil
}

// Process processes a RTP packet.
func (p *Cleaner) Process(pkt *rtp.Packet) ([]*Output, error) {
	// remove padding
	pkt.Header.Padding = false
	pkt.PaddingSize = 0

	if p.h264Decoder != nil {
		return p.processH264(pkt)
	}

	return []*Output{{
		Packet:       pkt,
		PTSEqualsDTS: true,
	}}, nil
}
