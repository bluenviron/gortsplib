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
		// Create TLS listener for RTSPS
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		require.NoError(t, err)

		l, err := tls.Listen("tcp", "localhost:0", &tls.Config{Certificates: []tls.Certificate{cert}})
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()

		go func() {
			nconn, err2 := l.Accept()
			require.NoError(t, err2)
			handleServerConnection(t, serverDone, nconn)
		}()

		// Create a media with secure profile (SAVP)
		media := &description.Media{
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

		desc := &description.Session{
			Medias: []*description.Media{media},
		}

		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme:    u.Scheme,
			Host:      u.Host,
			TLSConfig: &tls.Config{InsecureSkipVerify: true},
		}

		// Force TCP protocol for this test
		protocol := ProtocolTCP
		c.Protocol = &protocol

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - TCP+RTSPS with secure profile sets secure=true
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("TCP+RTSPS with non-secure profile - secure=false", func(t *testing.T) {
		// Create TLS listener for RTSPS
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		require.NoError(t, err)

		l, err := tls.Listen("tcp", "localhost:0", &tls.Config{Certificates: []tls.Certificate{cert}})
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()

		go func() {
			nconn, err2 := l.Accept()
			require.NoError(t, err2)
			handleServerConnection(t, serverDone, nconn)
		}()

		// Create a media with NON-secure profile (default RTP/AVP)
		media := &description.Media{
			Type: description.MediaTypeVideo,
			// Profile defaults to RTP/AVP which is NOT secure
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

		desc := &description.Session{
			Medias: []*description.Media{media},
		}

		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme:    u.Scheme,
			Host:      u.Host,
			TLSConfig: &tls.Config{InsecureSkipVerify: true},
		}

		// Force TCP protocol for this test
		protocol := ProtocolTCP
		c.Protocol = &protocol

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - TCP+RTSPS with non-secure profile sets secure=false
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("UDP+RTSPS with any profile - secure=true", func(t *testing.T) {
		// Create TLS listener for RTSPS
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		require.NoError(t, err)

		l, err := tls.Listen("tcp", "localhost:0", &tls.Config{Certificates: []tls.Certificate{cert}})
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()

		go func() {
			nconn, err2 := l.Accept()
			require.NoError(t, err2)
			handleServerConnection(t, serverDone, nconn)
		}()

		// Create a media with secure profile (SAVP)
		media := &description.Media{
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

		desc := &description.Session{
			Medias: []*description.Media{media},
		}

		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme:    u.Scheme,
			Host:      u.Host,
			TLSConfig: &tls.Config{InsecureSkipVerify: true},
		}

		// Force UDP protocol (default, not TCP) for this test
		protocol := ProtocolUDP
		c.Protocol = &protocol

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - UDP+RTSPS always sets secure=true regardless of media profile
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("TCP+RTSP with any profile - secure=false", func(t *testing.T) {
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

		// Create a media with regular profile (RTP/AVP - not secure)
		media := &description.Media{
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

		desc := &description.Session{
			Medias: []*description.Media{media},
		}

		u, err := base.ParseURL("rtsp://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme: u.Scheme,
			Host:   u.Host,
		}

		// Force TCP protocol for this test
		protocol := ProtocolTCP
		c.Protocol = &protocol

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - TCP+RTSP always sets secure=false regardless of media profile
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("UDP+RTSP with any profile - secure=false", func(t *testing.T) {
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

		// Create a media with secure profile (just to show it doesn't matter for UDP+RTSP)
		media := &description.Media{
			Type:    description.MediaTypeVideo,
			Profile: headers.TransportProfileSAVP, // This is secure but won't affect the result
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

		desc := &description.Session{
			Medias: []*description.Media{media},
		}

		u, err := base.ParseURL("rtsp://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme: u.Scheme,
			Host:   u.Host,
		}

		// Force UDP protocol (default, not TCP) for this test
		protocol := ProtocolUDP
		c.Protocol = &protocol

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - UDP+RTSP always sets secure=false regardless of media profile
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("TCP+RTSPS with mixed profiles - secure=true if any profile is secure", func(t *testing.T) {
		// Create TLS listener for RTSPS
		cert, err := tls.X509KeyPair(serverCert, serverKey)
		require.NoError(t, err)

		l, err := tls.Listen("tcp", "localhost:0", &tls.Config{Certificates: []tls.Certificate{cert}})
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()

		go func() {
			nconn, err2 := l.Accept()
			require.NoError(t, err2)
			handleServerConnection(t, serverDone, nconn)
		}()

		// Create multiple medias: one secure, one non-secure
		mediaSecure := &description.Media{
			Type:    description.MediaTypeVideo,
			Profile: headers.TransportProfileSAVP, // This is secure
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

		mediaNonSecure := &description.Media{
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

		desc := &description.Session{
			Medias: []*description.Media{mediaSecure, mediaNonSecure},
		}

		u, err := base.ParseURL("rtsps://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme:    u.Scheme,
			Host:      u.Host,
			TLSConfig: &tls.Config{InsecureSkipVerify: true},
		}

		// Force TCP protocol for this test
		protocol := ProtocolTCP
		c.Protocol = &protocol

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed - TCP+RTSPS with mixed profiles, one secure sets secure=true
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})
}
