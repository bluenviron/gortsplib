// This example shows how to:
// - connect to a RTSP server using RTSP-over-HTTP tunneling (Apple standard)
// - get and print transport-level statistics
package main

import (
	// Uncomment when using TLS
	// "crypto/tls"
	"log"
	"os"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
)

// This example shows how to connect to a RTSP server using RTSP-over-HTTP tunneling (Apple standard)
func main() {
	if len(os.Args) < 2 {
		log.Printf("Usage: %s <rtsp url>\n", os.Args[0])
		os.Exit(1)
	}

	// Create a client that uses HTTP tunneling
	httpTransport := gortsplib.TransportHTTP
	c := &gortsplib.Client{
		Transport: &httpTransport,
		// Optional TLS configuration for HTTPS connections
		// Uncomment if connecting via HTTPS
		/*
		TLSConfig: &tls.Config{
			// For testing only - in production, don't skip verification
			// InsecureSkipVerify: true,
		},
		*/
	}

	// Parse the URL
	u, err := base.ParseURL(os.Args[1])
	if err != nil {
		log.Printf("Error parsing URL: %v", err)
		os.Exit(1)
	}

	// Connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		log.Printf("Error connecting to server: %v", err)
		os.Exit(1)
	}
	defer c.Close()

	// Set a handler for when a packet arrives
	c.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		log.Printf("RTP packet from %s, payload type %d, %d bytes", medi.Type, pkt.PayloadType, len(pkt.Payload))
	})

	// Set a handler for when an RTCP packet arrives
	c.OnPacketRTCPAny(func(medi *description.Media, pkt rtcp.Packet) {
		log.Printf("RTCP packet from %s, type %T", medi.Type, pkt)
	})

	// Get stream description
	desc, _, err := c.Describe(u)
	if err != nil {
		log.Printf("Error describing: %v", err)
		os.Exit(1)
	}

	// Set up all medias
	err = c.SetupAll(u, desc.Medias)
	if err != nil {
		log.Printf("Error setting up medias: %v", err)
		os.Exit(1)
	}

	// Start playing
	_, err = c.Play(nil)
	if err != nil {
		log.Printf("Error playing: %v", err)
		os.Exit(1)
	}

	// Print transport statistics every 5 seconds
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	for range t.C {
		log.Printf("Statistics: %+v", c.Stats())
	}
}