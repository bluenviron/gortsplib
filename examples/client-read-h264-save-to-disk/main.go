package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/aler9/gortsplib/pkg/rtph264"
	"github.com/asticode/go-astits"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check whether there's a H264 track
// 3. save the content of the H264 track to a file in MPEG-TS format

const (
	inputStream = "rtsp://localhost:8554/mystream"
	outputFile  = "mystream.ts"
)

func main() {
	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead(inputStream)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// find the H264 track
	var h264TrackID int = -1
	var h264Conf *gortsplib.TrackConfigH264
	for i, track := range conn.Tracks() {
		if track.IsH264() {
			h264TrackID = i
			h264Conf, err = track.ExtractConfigH264()
			if err != nil {
				panic(err)
			}
		}
	}
	if h264TrackID < 0 {
		panic(fmt.Errorf("H264 track not found"))
	}

	// open output file
	f, err := os.Create(outputFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// istantiate things needed to decode RTP/H264 and encode MPEG-TS
	b := bufio.NewWriter(f)
	defer b.Flush()
	mux := astits.NewMuxer(context.Background(), b)
	dec := rtph264.NewDecoder()
	dtsEst := h264.NewDTSEstimator()
	firstPacketWritten := false
	var startPTS time.Duration

	// add an H264 track to the MPEG-TS muxer
	mux.AddElementaryStream(astits.PMTElementaryStream{
		ElementaryPID: 256,
		StreamType:    astits.StreamTypeH264Video,
	})
	mux.SetPCRPID(256)

	// read packets
	err = conn.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
		if trackID != h264TrackID {
			return
		}

		if streamType != gortsplib.StreamTypeRTP {
			return
		}

		// parse RTP packets
		var pkt rtp.Packet
		err := pkt.Unmarshal(payload)
		if err != nil {
			return
		}

		// decode H264 NALUs from RTP packets
		nalus, pts, err := dec.DecodeUntilMarker(&pkt)
		if err != nil {
			return
		}

		if !firstPacketWritten {
			firstPacketWritten = true
			startPTS = pts
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
				filteredNALUs = append(filteredNALUs, h264Conf.SPS)
				filteredNALUs = append(filteredNALUs, h264Conf.PPS)
			}

			filteredNALUs = append(filteredNALUs, nalu)
		}

		// encode into Annex-B
		enc, err := h264.EncodeAnnexB(filteredNALUs)
		if err != nil {
			panic(err)
		}

		dts := dtsEst.Feed(pts - startPTS)
		pts = pts - startPTS

		// write TS packet
		_, err = mux.WriteData(&astits.MuxerData{
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
			panic(err)
		}

		fmt.Println("wrote ts packet")
	})
	panic(err)
}
