package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph264"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a H264 stream and an MPEG-4 audio format
// 3. save the content of those formats in a file in MPEG-TS format

func main() {
	c := gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/stream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// find available medias
	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 media and format
	var h264Format *format.H264
	h264Media := desc.FindFormat(&h264Format)
	if h264Media == nil {
		panic("H264 media not found")
	}

	// find the MPEG-4 audio media and format
	var mpeg4AudioFormat *format.MPEG4Audio
	mpeg4AudioMedia := desc.FindFormat(&mpeg4AudioFormat)
	if mpeg4AudioMedia == nil {
		panic("MPEG-4 audio media not found")
	}

	// setup RTP -> H264 decoder
	h264RTPDec, err := h264Format.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup RTP -> MPEG-4 audio decoder
	mpeg4AudioRTPDec, err := mpeg4AudioFormat.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup MPEG-TS muxer
	mpegtsMuxer := &mpegtsMuxer{
		fileName:         "mystream.ts",
		h264Format:       h264Format,
		mpeg4AudioFormat: mpeg4AudioFormat,
	}
	err = mpegtsMuxer.initialize()
	if err != nil {
		panic(err)
	}
	defer mpegtsMuxer.close()

	// setup all medias
	err = c.SetupAll(desc.BaseURL, desc.Medias)
	if err != nil {
		panic(err)
	}

	// called when a H264/RTP packet arrives
	c.OnPacketRTP(h264Media, h264Format, func(pkt *rtp.Packet) {
		// decode timestamp
		pts, ok := c.PacketPTS2(h264Media, pkt)
		if !ok {
			log.Printf("waiting for timestamp")
			return
		}

		// extract access unit from RTP packets
		au, err := h264RTPDec.Decode(pkt)
		if err != nil {
			if err != rtph264.ErrNonStartingPacketAndNoPrevious && err != rtph264.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		// encode the access unit into MPEG-TS
		err = mpegtsMuxer.writeH264(au, pts)
		if err != nil {
			log.Printf("ERR: %v", err)
			return
		}

		log.Printf("saved TS packet")
	})

	// called when a MPEG-4 audio / RTP packet arrives
	c.OnPacketRTP(mpeg4AudioMedia, mpeg4AudioFormat, func(pkt *rtp.Packet) {
		// decode timestamp
		pts, ok := c.PacketPTS2(mpeg4AudioMedia, pkt)
		if !ok {
			log.Printf("waiting for timestamp")
			return
		}

		// extract access units from RTP packets
		aus, err := mpeg4AudioRTPDec.Decode(pkt)
		if err != nil {
			log.Printf("ERR: %v", err)
			return
		}

		// encode access units into MPEG-TS
		err = mpegtsMuxer.writeMPEG4Audio(aus, pts)
		if err != nil {
			log.Printf("ERR: %v", err)
			return
		}

		log.Printf("saved TS packet")
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
