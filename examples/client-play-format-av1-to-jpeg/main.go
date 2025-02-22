//go:build cgo

package main

import (
	"image"
	"image/jpeg"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpav1"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/av1"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a AV1 stream
// 3. decode the AV1 stream into RGBA frames
// 4. convert RGBA frames to JPEG images and save them on disk

// This example requires the FFmpeg libraries, that can be installed with this command:
// apt install -y libavcodec-dev libswscale-dev gcc pkg-config

func saveToFile(img image.Image) error {
	// create file
	fname := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10) + ".jpg"
	f, err := os.Create(fname)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	log.Println("saving", fname)

	// convert to jpeg
	return jpeg.Encode(f, img, &jpeg.Options{
		Quality: 60,
	})
}

func main() {
	c := gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/mystream")
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

	// find the AV1 media and format
	var forma *format.AV1
	medi := desc.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP -> AV1 decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup AV1 -> RGBA decoder
	av1Dec := &av1Decoder{}
	err = av1Dec.initialize()
	if err != nil {
		panic(err)
	}
	defer av1Dec.close()

	// setup a single media
	_, err = c.Setup(desc.BaseURL, medi, 0, 0)
	if err != nil {
		panic(err)
	}

	firstRandomReceived := false
	saveCount := 0

	// called when a RTP packet arrives
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		// extract AV1 temporal units from RTP packets
		tu, err := rtpDec.Decode(pkt)
		if err != nil {
			if err != rtpav1.ErrNonStartingPacketAndNoPrevious && err != rtpav1.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		// wait for a random access unit
		if !firstRandomReceived && !av1.IsRandomAccess2(tu) {
			log.Printf("waiting for a random access unit")
			return
		}
		firstRandomReceived = true

		// convert AV1 temporal units into RGBA frames
		img, err := av1Dec.decode(tu)
		if err != nil {
			panic(err)
		}

		// wait for a frame
		if img == nil {
			return
		}

		// convert frame to JPEG and save to file
		err = saveToFile(img)
		if err != nil {
			panic(err)
		}

		saveCount++
		if saveCount == 5 {
			log.Printf("saved 5 images, exiting")
			os.Exit(1)
		}
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
