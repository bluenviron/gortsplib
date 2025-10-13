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
	// Test that secure profiles (SAVP) require RTSPS connection
	t.Run("secure profile with rtsp scheme should fail", func(t *testing.T) {
		l, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()

		go func() {
			defer close(serverDone)

			nconn, err2 := l.Accept()
			if err2 != nil {
				return // Client closes connection early due to validation error
			}
			defer nconn.Close()
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			req, err2 := conn.ReadRequest()
			if err2 != nil {
				return // Client closes connection early
			}
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
			if err2 != nil {
				return // Client might close connection
			}
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

		u, err := base.ParseURL("rtsp://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme: u.Scheme,
			Host:   u.Host,
		}

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should fail because we're using SAVP profile with rtsp scheme
		_, err = c.Announce(u, desc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "secure profiles require RTSPS connection")
		require.Contains(t, err.Error(), "Profile [1]") // Profile value is numeric
		require.Contains(t, err.Error(), "ID: [1]")
		require.Contains(t, err.Error(), "Control [trackID=0]")
	})

	t.Run("secure profile with rtsps scheme should succeed", func(t *testing.T) {
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

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed because we're using SAVP profile with rtsps scheme
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("non-secure profile with rtsp scheme should succeed", func(t *testing.T) {
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

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should succeed because we're using regular profile with rtsp scheme
		_, err = c.Announce(u, desc)
		require.NoError(t, err)
	})

	t.Run("mixed profiles with one secure should fail on rtsp", func(t *testing.T) {
		l, err := net.Listen("tcp", "localhost:0")
		require.NoError(t, err)
		defer l.Close()

		serverDone := make(chan struct{})
		defer func() { <-serverDone }()

		go func() {
			defer close(serverDone)

			nconn, err2 := l.Accept()
			if err2 != nil {
				return // Client closes connection early due to validation error
			}
			defer nconn.Close()
			conn := conn.NewConn(bufio.NewReader(nconn), nconn)

			req, err2 := conn.ReadRequest()
			if err2 != nil {
				return // Client closes connection early
			}
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
			if err2 != nil {
				return // Client might close connection
			}
		}()

		// Create multiple medias with mixed profiles
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

		u, err := base.ParseURL("rtsp://" + l.Addr().String() + "/teststream")
		require.NoError(t, err)

		c := Client{
			Scheme: u.Scheme,
			Host:   u.Host,
		}

		err = c.Start()
		require.NoError(t, err)
		defer c.Close()

		// This should fail because one media uses SAVP profile with rtsp scheme
		_, err = c.Announce(u, desc)
		require.Error(t, err)
		require.Contains(t, err.Error(), "secure profiles require RTSPS connection")
		require.Contains(t, err.Error(), "Profile [1]") // Profile value is numeric
		require.Contains(t, err.Error(), "ID: [1]")
	})
}
