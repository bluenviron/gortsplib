//go:build enable_e2e_tests

package teste2e

import (
	"crypto/tls"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func multicastCapableIP(t *testing.T) string {
	intfs, err := net.Interfaces()
	require.NoError(t, err)

	for _, intf := range intfs {
		if (intf.Flags & net.FlagMulticast) != 0 {
			addrs, err := intf.Addrs()
			if err != nil {
				continue
			}

			for _, addr := range addrs {
				switch v := addr.(type) {
				case *net.IPNet:
					return v.IP.String()
				case *net.IPAddr:
					return v.IP.String()
				}
			}
		}
	}

	t.Errorf("unable to find a multicast IP")
	return ""
}

func TestClientVsServer(t *testing.T) {
	for _, ca := range []struct {
		publisherScheme string
		publisherProto  string
		publisherTunnel string
		readerScheme    string
		readerProto     string
		readerTunnel    string
	}{
		{
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherTunnel: "none",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherTunnel: "none",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherTunnel: "none",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherTunnel: "none",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherTunnel: "none",
			readerScheme:    "rtsp",
			readerProto:     "multicast",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsps",
			publisherProto:  "tcp",
			publisherTunnel: "none",
			readerScheme:    "rtsps",
			readerProto:     "tcp",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsps",
			publisherProto:  "udp",
			publisherTunnel: "none",
			readerScheme:    "rtsps",
			readerProto:     "tcp",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsps",
			publisherProto:  "udp",
			publisherTunnel: "none",
			readerScheme:    "rtsps",
			readerProto:     "multicast",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherTunnel: "http",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerTunnel:    "none",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherTunnel: "none",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerTunnel:    "http",
		},
	} {
		t.Run(strings.Join([]string{
			ca.publisherScheme,
			ca.publisherProto,
			ca.publisherTunnel,
			ca.readerScheme,
			ca.readerProto,
			ca.readerTunnel,
		}, "_"), func(t *testing.T) {
			ss := &sampleServer{}

			if ca.publisherScheme == "rtsps" {
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				ss.tlsConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			err := ss.initialize()
			require.NoError(t, err)
			defer ss.close()

			desc := &description.Session{
				Medias: []*description.Media{
					{
						Type: description.MediaTypeVideo,
						Formats: []format.Format{&format.H264{
							PayloadTyp:        96,
							PacketizationMode: 1,
						}},
					},
				},
			}

			var publisherTunnel gortsplib.Tunnel
			if ca.publisherTunnel == "http" {
				publisherTunnel = gortsplib.TunnelHTTP
			} else {
				publisherTunnel = gortsplib.TunnelNone
			}

			var publisherProto gortsplib.Protocol
			switch ca.publisherProto {
			case "udp":
				publisherProto = gortsplib.ProtocolUDP
			case "tcp":
				publisherProto = gortsplib.ProtocolTCP
			}

			publisher := &gortsplib.Client{
				TLSConfig: &tls.Config{InsecureSkipVerify: true},
				Tunnel:    publisherTunnel,
				Protocol:  &publisherProto,
			}
			err = publisher.StartRecording(ca.publisherScheme+"://127.0.0.1:8554/test/stream?key=val", desc)
			require.NoError(t, err)
			defer publisher.Close()

			time.Sleep(1 * time.Second)

			var readerTunnel gortsplib.Tunnel
			if ca.readerTunnel == "http" {
				readerTunnel = gortsplib.TunnelHTTP
			} else {
				readerTunnel = gortsplib.TunnelNone
			}

			var readerProto gortsplib.Protocol
			switch ca.readerProto {
			case "udp":
				readerProto = gortsplib.ProtocolUDP
			case "tcp":
				readerProto = gortsplib.ProtocolTCP
			case "multicast":
				readerProto = gortsplib.ProtocolUDPMulticast
			}

			u, err := base.ParseURL(ca.readerScheme + "://" + multicastCapableIP(t) + ":8554/test/stream?key=val")
			require.NoError(t, err)

			reader := &gortsplib.Client{
				Scheme:    u.Scheme,
				Host:      u.Host,
				TLSConfig: &tls.Config{InsecureSkipVerify: true},
				Tunnel:    readerTunnel,
				Protocol:  &readerProto,
			}
			err = reader.Start()
			require.NoError(t, err)
			defer reader.Close()

			desc2, _, err := reader.Describe(u)
			require.NoError(t, err)

			err = reader.SetupAll(desc2.BaseURL, desc2.Medias)
			require.NoError(t, err)

			packetRecv := make(chan struct{})

			reader.OnPacketRTPAny(func(medi *description.Media, _ format.Format, _ *rtp.Packet) {
				close(packetRecv)
			})

			_, err = reader.Play(nil)
			require.NoError(t, err)

			err = publisher.WritePacketRTP(desc.Medias[0], &rtp.Packet{
				Header: rtp.Header{
					PayloadType:    96,
					SequenceNumber: 1,
					SSRC:           123,
				},
				Payload: []byte{5},
			})
			require.NoError(t, err)

			<-packetRecv
		})
	}
}
