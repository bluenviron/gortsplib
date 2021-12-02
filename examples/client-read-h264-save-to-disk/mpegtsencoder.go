package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/asticode/go-astits"
)

// mpegtsEncoder allows to encode H264 NALUs into MPEG-TS.
type mpegtsEncoder struct {
	f                  *os.File
	b                  *bufio.Writer
	mux                *astits.Muxer
	dtsEst             *h264.DTSEstimator
	firstPacketWritten bool
	startPTS           time.Duration
	h264Conf           *gortsplib.TrackConfigH264
}

// newMPEGTSEncoder allocates a mpegtsEncoder.
func newMPEGTSEncoder(h264Conf *gortsplib.TrackConfigH264) (*mpegtsEncoder, error) {
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
		f:        f,
		b:        b,
		mux:      mux,
		dtsEst:   h264.NewDTSEstimator(),
		h264Conf: h264Conf,
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

	// check whether there's an IDR
	idrPresent := func() bool {
		for _, nalu := range nalus {
			typ := h264.NALUType(nalu[0] & 0x1F)
			if typ == h264.NALUTypeIDR {
				return true
			}
		}
		return false
	}()

	// prepend an AUD. This is required by some players
	filteredNALUs := [][]byte{
		{byte(h264.NALUTypeAccessUnitDelimiter), 240},
	}

	for _, nalu := range nalus {
		// remove existing SPS, PPS, AUD
		typ := h264.NALUType(nalu[0] & 0x1F)
		switch typ {
		case h264.NALUTypeSPS, h264.NALUTypePPS, h264.NALUTypeAccessUnitDelimiter:
			continue
		}

		// add SPS and PPS before every IDR
		if typ == h264.NALUTypeIDR {
			filteredNALUs = append(filteredNALUs, e.h264Conf.SPS, e.h264Conf.PPS)
		}

		filteredNALUs = append(filteredNALUs, nalu)
	}

	// encode into Annex-B
	enc, err := h264.EncodeAnnexB(filteredNALUs)
	if err != nil {
		return err
	}

	pts -= e.startPTS
	dts := e.dtsEst.Feed(pts)

	// write TS packet
	_, err = e.mux.WriteData(&astits.MuxerData{
		PID: 256,
		AdaptationField: &astits.PacketAdaptationField{
			RandomAccessIndicator: idrPresent,
		},
		PES: &astits.PESData{
			Header: &astits.PESHeader{
				OptionalHeader: &astits.PESOptionalHeader{
					MarkerBits:      2,
					PTSDTSIndicator: astits.PTSDTSIndicatorBothPresent,
					DTS:             &astits.ClockReference{Base: int64(dts.Seconds() * 90000)},
					PTS:             &astits.ClockReference{Base: int64(pts.Seconds() * 90000)},
				},
				StreamID: 224, // video
			},
			Data: enc,
		},
	})
	if err != nil {
		return err
	}

	fmt.Println("wrote TS packet")
	return nil
}
