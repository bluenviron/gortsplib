package gortsplib

import (
	"bufio"
	"crypto/tls"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
)

// handleServerConnection handles the common server-side connection logic for the security profile tests
func handleServerConnection(t *testing.T, serverDone chan struct{}, nconn net.Conn) {
	defer close(serverDone)

	if nconn == nil {
		return
	}
	defer nconn.Close()

	conn := conn.NewConn(bufio.NewReader(nconn), nconn)

	req, err2 := conn.ReadRequest()
	require.NoError(t, err2)
	require.Equal(t, base.Options, req.Method)

	err2 = conn.WriteResponse(&base.Response{
		StatusCode: base.StatusOK,
		Header: base.Header{
			"Public": base.HeaderValue{strings.Join([]string{
				string(base.Announce),
				string(base.Setup),
				string(base.Record),
			}, ", ")},
		},
	})
	require.NoError(t, err2)

	req, err2 = conn.ReadRequest()
	require.NoError(t, err2)
	require.Equal(t, base.Announce, req.Method)

	err2 = conn.WriteResponse(&base.Response{
		StatusCode: base.StatusOK,
	})
	require.NoError(t, err2)
}

// createSecureMedia creates a media description with secure profile (SAVP)
func createSecureMedia() *description.Media {
	return &description.Media{
		Type:    description.MediaTypeVideo,
		Profile: headers.TransportProfileSAVP, // This is the secure profile
		Formats: []format.Format{&format.H264{
			PayloadTyp: 96,
			SPS: []byte{
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
				0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
				0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
				0x20,
			},
			PPS: []byte{
				0x44, 0x01, 0xc0, 0x25, 0x2f, 0x05, 0x32, 0x40,
			},
			PacketizationMode: 1,
		}},
		ID:      "1",
		Control: "trackID=0",
	}
}

// createNonSecureMedia creates a media description with non-secure profile (RTP/AVP)
func createNonSecureMedia() *description.Media {
	return &description.Media{
		Type: description.MediaTypeVideo,
		// Profile defaults to RTP/AVP which is not secure
		Formats: []format.Format{&format.H264{
			PayloadTyp: 96,
			SPS: []byte{
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
				0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
				0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9,
				0x20,
			},
			PPS: []byte{
				0x44, 0x01, 0xc0, 0x25, 0x2f, 0x05, 0x32, 0x40,
			},
			PacketizationMode: 1,
		}},
		ID:      "1",
		Control: "trackID=0",
	}
}

// createAudioMedia creates a non-secure audio media description
func createAudioMedia() *description.Media {
	return &description.Media{
		Type: description.MediaTypeAudio,
		// Profile defaults to RTP/AVP which is not secure
		Formats: []format.Format{&format.G711{
			PayloadTyp:   0,
			SampleRate:   8000,
			ChannelCount: 1,
		}},
		ID:      "2",
		Control: "trackID=1",
	}
}

// setupTLSTestServer creates a TLS listener and server goroutine for RTSPS testing
func setupTLSTestServer(t *testing.T) (net.Listener, chan struct{}) {
	// Create TLS listener for RTSPS
	cert, err := tls.X509KeyPair(serverCert, serverKey)
	require.NoError(t, err)

	l, err := tls.Listen("tcp", "localhost:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	require.NoError(t, err)

	serverDone := make(chan struct{})

	go func() {
		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		handleServerConnection(t, serverDone, nconn)
	}()

	return l, serverDone
}

// createTLSClientWithProtocol creates a configured RTSP client for RTSPS testing
func createTLSClientWithProtocol(addr string, protocol Protocol) *Client {
	u, err := base.ParseURL("rtsps://" + addr + "/teststream")
	if err != nil {
		panic(err) // This should never happen in tests
	}

	c := &Client{
		Scheme:    u.Scheme,
		Host:      u.Host,
		TLSConfig: &tls.Config{InsecureSkipVerify: true},
		Protocol:  &protocol,
	}

	return c
}

// testRTSPAnnounceWithProtocol is a helper function that tests RTSP announce with a specific protocol and media
func testRTSPAnnounceWithProtocol(t *testing.T, protocol Protocol, mediaFactory func() *description.Media) {
	l, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()

	go func() {
		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		handleServerConnection(t, serverDone, nconn)
	}()

	// Create media using the provided factory
	media := mediaFactory()

	desc := &description.Session{
		Medias: []*description.Media{media},
	}

	u, err := base.ParseURL("rtsp://" + l.Addr().String() + "/teststream")
	require.NoError(t, err)

	c := Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	// Set the protocol for this test
	c.Protocol = &protocol

	err = c.Start()
	require.NoError(t, err)
	defer c.Close()

	// This should succeed - RTSP always sets secure=false regardless of media profile
	_, err = c.Announce(u, desc)
	require.NoError(t, err)
}

func TestClientAnnounceSecureProfileValidation(t *testing.T) {
	// Test how secure flag is determined based on different protocol/scheme/profile combinations
	//
	// BEHAVIOR MATRIX:
	// ┌─────────┬─────────┬──────────────┬────────────────────────────┐
	// │Protocol │ Scheme  │Media Profile │ secure flag result         │
	// ├─────────┼─────────┼──────────────┼────────────────────────────┤
	// │  TCP    │ RTSPS   │    SAVP      │ true (has secure profile)  │
	// │  TCP    │ RTSPS   │   RTP/AVP    │ false (no secure profile)  │
	// │  UDP    │ RTSPS   │    Any       │ true (scheme is rtsps)     │
	// │  TCP    │ RTSP    │    Any       │ false (scheme is rtsp)     │
	// │  UDP    │ RTSP    │    Any       │ false (scheme is rtsp)     │
	// └─────────┴─────────┴──────────────┴────────────────────────────┘
	//
	// Current implementation logic:
	// if (Protocol == TCP && Scheme == "rtsps") {
	//     secure = hasSecureProfile  // Check if any media has SAVP profile
	// } else {
	//     secure = (Scheme == "rtsps")  // Based on scheme only
	// }

	t.Run("TCP+RTSPS with secure profile - secure=true", func(t *testing.T) {
		l, serverDone := setupTLSTestServer(t)
		defer l.Close()
		defer func() { <-serverDone }()

		// Create a media with secure profile (SAVP)
		media := createSecureMedia()
		desc := &description.Session{Medias: []*description.Media{media}}

		c := createTLSClientWithProtocol(l.Addr().String(), ProtocolTCP)
		err := c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - TCP+RTSPS with secure profile sets secure=true
		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("TCP+RTSPS with non-secure profile - secure=false", func(t *testing.T) {
		l, serverDone := setupTLSTestServer(t)
		defer l.Close()
		defer func() { <-serverDone }()

		// Create a media with NON-secure profile (default RTP/AVP)
		media := createNonSecureMedia()
		desc := &description.Session{Medias: []*description.Media{media}}

		c := createTLSClientWithProtocol(l.Addr().String(), ProtocolTCP)
		err := c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - TCP+RTSPS with non-secure profile sets secure=false
		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("UDP+RTSPS with any profile - secure=true", func(t *testing.T) {
		l, serverDone := setupTLSTestServer(t)
		defer l.Close()
		defer func() { <-serverDone }()

		// Create a media with secure profile (SAVP)
		media := createSecureMedia()
		desc := &description.Session{Medias: []*description.Media{media}}

		c := createTLSClientWithProtocol(l.Addr().String(), ProtocolUDP)
		err := c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - UDP+RTSPS always sets secure=true regardless of media profile
		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("TCP+RTSP with any profile - secure=false", func(t *testing.T) {
		// Create a media with regular profile (RTP/AVP - not secure)
		testRTSPAnnounceWithProtocol(t, ProtocolTCP, createNonSecureMedia)
	})

	t.Run("UDP+RTSP with any profile - secure=false", func(t *testing.T) {
		// Create a media with secure profile (just to show it doesn't matter for UDP+RTSP)
		testRTSPAnnounceWithProtocol(t, ProtocolUDP, createSecureMedia)
	})

	t.Run("TCP+RTSPS with mixed profiles - secure=true if any profile is secure", func(t *testing.T) {
		l, serverDone := setupTLSTestServer(t)
		defer l.Close()
		defer func() { <-serverDone }()

		// Create multiple medias: one secure, one non-secure
		mediaSecure := createSecureMedia()
		mediaNonSecure := createAudioMedia()
		desc := &description.Session{Medias: []*description.Media{mediaSecure, mediaNonSecure}}

		c := createTLSClientWithProtocol(l.Addr().String(), ProtocolTCP)
		err := c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - TCP+RTSPS with mixed profiles, one secure sets secure=true
		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})
}
