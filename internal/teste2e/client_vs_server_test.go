//go:build enable_e2e_tests

package teste2e

import (
	"crypto/tls"
	"net"
	"testing"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
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
		readerScheme    string
		readerProto     string
	}{
		{
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			readerScheme:    "rtsp",
			readerProto:     "udp",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			readerScheme:    "rtsp",
			readerProto:     "udp",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
		},
		{
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			readerScheme:    "rtsp",
			readerProto:     "multicast",
		},
		{
			publisherScheme: "rtsps",
			publisherProto:  "tcp",
			readerScheme:    "rtsps",
			readerProto:     "tcp",
		},
		{
			publisherScheme: "rtsps",
			publisherProto:  "udp",
			readerScheme:    "rtsps",
			readerProto:     "tcp",
		},
		{
			publisherScheme: "rtsps",
			publisherProto:  "udp",
			readerScheme:    "rtsps",
			readerProto:     "multicast",
		},
	} {
		t.Run(ca.publisherScheme+"_"+ca.publisherProto+"_"+
			ca.readerScheme+"_"+ca.readerProto, func(t *testing.T) {
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

			var publisherProto gortsplib.Transport
			switch ca.publisherProto {
			case "udp":
				publisherProto = gortsplib.TransportUDP
			case "tcp":
				publisherProto = gortsplib.TransportTCP
			}

			publisher := &gortsplib.Client{
				TLSConfig: &tls.Config{InsecureSkipVerify: true},
				Transport: &publisherProto,
			}
			err = publisher.StartRecording(ca.publisherScheme+"://127.0.0.1:8554/test/stream?key=val", desc)
			require.NoError(t, err)
			defer publisher.Close()

			time.Sleep(1 * time.Second)

			var readerProto gortsplib.Transport
			switch ca.readerProto {
			case "udp":
				readerProto = gortsplib.TransportUDP
			case "tcp":
				readerProto = gortsplib.TransportTCP
			case "multicast":
				readerProto = gortsplib.TransportUDPMulticast
			}

			u, err := base.ParseURL(ca.readerScheme + "://" + multicastCapableIP(t) + ":8554/test/stream?key=val")
			require.NoError(t, err)

			reader := &gortsplib.Client{
				Scheme:    u.Scheme,
				Host:      u.Host,
				TLSConfig: &tls.Config{InsecureSkipVerify: true},
				Transport: &readerProto,
			}
			err = reader.Start2()
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
