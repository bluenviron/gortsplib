package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"time"

	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/asticode/go-astits"
)

// mpegtsEncoder allows to encode H264 NALUs into MPEG-TS.
type mpegtsEncoder struct {
	sps []byte
	pps []byte

	f                  *os.File
	b                  *bufio.Writer
	mux                *astits.Muxer
	dtsEst             *h264.DTSEstimator
	firstPacketWritten bool
	startPTS           time.Duration
}

// newMPEGTSEncoder allocates a mpegtsEncoder.
func newMPEGTSEncoder(sps []byte, pps []byte) (*mpegtsEncoder, error) {
	f, err := os.Create("mystream.ts")
	if err != nil {
		return nil, err
	}
	b := bufio.NewWriter(f)

	mux := astits.NewMuxer(context.Background(), b)
	mux.AddElementaryStream(astits.PMTElementaryStream{
		ElementaryPID: 256,
		StreamType:    astits.StreamTypeH264Video,
	})
	mux.SetPCRPID(256)

	return &mpegtsEncoder{
		sps:    sps,
		pps:    pps,
		f:      f,
		b:      b,
		mux:    mux,
		dtsEst: h264.NewDTSEstimator(),
	}, nil
}

// close closes all the mpegtsEncoder resources.
func (e *mpegtsEncoder) close() {
	e.b.Flush()
	e.f.Close()
}

// encode encodes H264 NALUs into MPEG-TS.
func (e *mpegtsEncoder) encode(nalus [][]byte, pts time.Duration) error {
	if !e.firstPacketWritten {
		e.firstPacketWritten = true
		e.startPTS = pts
	}

	// prepend an AUD. This is required by some players
	filteredNALUs := [][]byte{
		{byte(h264.NALUTypeAccessUnitDelimiter), 240},
	}

	for _, nalu := range nalus {
		typ := h264.NALUType(nalu[0] & 0x1F)
		switch typ {
		case h264.NALUTypeSPS:
			e.sps = append([]byte(nil), nalu...)

		case h264.NALUTypePPS:
			e.pps = append([]byte(nil), nalu...)

		case h264.NALUTypeAccessUnitDelimiter, h264.NALUTypeSEI:
			continue

		case h264.NALUTypeIDR:
			// add SPS and PPS before every IDR
			if e.sps != nil && e.pps != nil {
				filteredNALUs = append(filteredNALUs, e.sps, e.pps)
			}
		}

		filteredNALUs = append(filteredNALUs, nalu)
	}

	// it's useless to go on if SPS or PPS have not been provided yet
	if e.sps == nil || e.pps == nil {
		return nil
	}

	// encode into Annex-B
	enc, err := h264.EncodeAnnexB(filteredNALUs)
	if err != nil {
		return err
	}

	pts -= e.startPTS
	dts := e.dtsEst.Feed(pts)

	oh := &astits.PESOptionalHeader{
		MarkerBits: 2,
	}

	if dts == pts {
		oh.PTSDTSIndicator = astits.PTSDTSIndicatorOnlyPTS
		oh.PTS = &astits.ClockReference{Base: int64(pts.Seconds() * 90000)}
	} else {
		oh.PTSDTSIndicator = astits.PTSDTSIndicatorBothPresent
		oh.DTS = &astits.ClockReference{Base: int64(dts.Seconds() * 90000)}
		oh.PTS = &astits.ClockReference{Base: int64(pts.Seconds() * 90000)}
	}

	// write TS packet
	_, err = e.mux.WriteData(&astits.MuxerData{
		PID: 256,
		AdaptationField: &astits.PacketAdaptationField{
			RandomAccessIndicator: h264.IDRPresent(filteredNALUs),
		},
		PES: &astits.PESData{
			Header: &astits.PESHeader{
				OptionalHeader: oh,
				StreamID:       224, // video
			},
			Data: enc,
		},
	})
	if err != nil {
		return err
	}

	log.Println("wrote TS packet")
	return nil
}
