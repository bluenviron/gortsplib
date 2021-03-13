package gortsplib

import (
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/rtph264"
)

func TestClientConnPublishSerial(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
			require.NoError(t, err)
			defer cnt1.close()

			time.Sleep(1 * time.Second)

			pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
			require.NoError(t, err)
			defer pc.Close()

			cnt2, err := newContainer("gstreamer", "source", []string{
				"filesrc location=emptyvideo.ts ! tsdemux ! video/x-h264" +
					" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=" + strconv.FormatInt(int64(pc.LocalAddr().(*net.UDPAddr).Port), 10),
			})
			require.NoError(t, err)
			defer cnt2.close()

			decoder := rtph264.NewDecoder()
			sps, pps, err := decoder.ReadSPSPPS(rtph264.PacketConnReader{pc}) //nolint:govet
			require.NoError(t, err)

			track, err := NewTrackH264(96, sps, pps)
			require.NoError(t, err)

			conf := ClientConf{
				StreamProtocol: func() *StreamProtocol {
					if proto == "udp" {
						v := StreamProtocolUDP
						return &v
					}
					v := StreamProtocolTCP
					return &v
				}(),
			}

			conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			buf := make([]byte, 2048)
			n, _, err := pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.ID, StreamTypeRTP, buf[:n])
			require.NoError(t, err)

			conn.Close()

			n, _, err = pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.ID, StreamTypeRTP, buf[:n])
			require.Error(t, err)
		})
	}
}

func TestClientConnPublishParallel(t *testing.T) {
	for _, ca := range []struct {
		proto  string
		server string
	}{
		{"udp", "rtsp-simple-server"},
		{"udp", "ffmpeg"},
		{"tcp", "rtsp-simple-server"},
		{"tcp", "ffmpeg"},
	} {
		t.Run(ca.proto+"_"+ca.server, func(t *testing.T) {
			switch ca.server {
			case "rtsp-simple-server":
				cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
				require.NoError(t, err)
				defer cnt1.close()

			default:
				cnt0, err := newContainer("rtsp-simple-server", "server0", []string{"{}"})
				require.NoError(t, err)
				defer cnt0.close()

				cnt1, err := newContainer("ffmpeg", "server", []string{
					"-fflags nobuffer -re -rtsp_flags listen -i rtsp://localhost:8555/teststream -c copy -f rtsp rtsp://localhost:8554/teststream",
				})
				require.NoError(t, err)
				defer cnt1.close()
			}

			time.Sleep(1 * time.Second)

			pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
			require.NoError(t, err)
			defer pc.Close()

			cnt2, err := newContainer("gstreamer", "source", []string{
				"filesrc location=emptyvideo.ts ! tsdemux ! video/x-h264" +
					" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=" + strconv.FormatInt(int64(pc.LocalAddr().(*net.UDPAddr).Port), 10),
			})
			require.NoError(t, err)
			defer cnt2.close()

			decoder := rtph264.NewDecoder()
			sps, pps, err := decoder.ReadSPSPPS(rtph264.PacketConnReader{pc}) //nolint:govet
			require.NoError(t, err)

			track, err := NewTrackH264(96, sps, pps)
			require.NoError(t, err)

			writerDone := make(chan struct{})
			defer func() { <-writerDone }()

			var conn *ClientConn
			defer func() { conn.Close() }()

			conf := ClientConf{
				StreamProtocol: func() *StreamProtocol {
					if ca.proto == "udp" {
						v := StreamProtocolUDP
						return &v
					}
					v := StreamProtocolTCP
					return &v
				}(),
			}

			go func() {
				defer close(writerDone)

				port := "8554"
				if ca.server == "ffmpeg" {
					port = "8555"
				}
				var err error
				conn, err = conf.DialPublish("rtsp://localhost:"+port+"/teststream",
					Tracks{track})
				require.NoError(t, err)

				buf := make([]byte, 2048)
				for {
					n, _, err := pc.ReadFrom(buf)
					if err != nil {
						break
					}

					err = conn.WriteFrame(track.ID, StreamTypeRTP, buf[:n])
					if err != nil {
						break
					}
				}
			}()

			if ca.server == "ffmpeg" {
				time.Sleep(5 * time.Second)
			}
			time.Sleep(1 * time.Second)

			cnt3, err := newContainer("ffmpeg", "read", []string{
				"-rtsp_transport", "udp",
				"-i", "rtsp://localhost:8554/teststream",
				"-vframes", "1",
				"-f", "image2",
				"-y", "/dev/null",
			})
			require.NoError(t, err)
			defer cnt3.close()

			code := cnt3.wait()
			require.Equal(t, 0, code)
		})
	}
}

func TestClientConnPublishPauseSerial(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
			require.NoError(t, err)
			defer cnt1.close()

			time.Sleep(1 * time.Second)

			pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
			require.NoError(t, err)
			defer pc.Close()

			cnt2, err := newContainer("gstreamer", "source", []string{
				"filesrc location=emptyvideo.ts ! tsdemux ! video/x-h264" +
					" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=" + strconv.FormatInt(int64(pc.LocalAddr().(*net.UDPAddr).Port), 10),
			})
			require.NoError(t, err)
			defer cnt2.close()

			decoder := rtph264.NewDecoder()
			sps, pps, err := decoder.ReadSPSPPS(rtph264.PacketConnReader{pc}) //nolint:govet
			require.NoError(t, err)

			track, err := NewTrackH264(96, sps, pps)
			require.NoError(t, err)

			conf := ClientConf{
				StreamProtocol: func() *StreamProtocol {
					if proto == "udp" {
						v := StreamProtocolUDP
						return &v
					}
					v := StreamProtocolTCP
					return &v
				}(),
			}

			conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer conn.Close()

			buf := make([]byte, 2048)

			n, _, err := pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.ID, StreamTypeRTP, buf[:n])
			require.NoError(t, err)

			_, err = conn.Pause()
			require.NoError(t, err)

			n, _, err = pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.ID, StreamTypeRTP, buf[:n])
			require.Error(t, err)

			_, err = conn.Record()
			require.NoError(t, err)

			n, _, err = pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.ID, StreamTypeRTP, buf[:n])
			require.NoError(t, err)
		})
	}
}

func TestClientConnPublishPauseParallel(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
			require.NoError(t, err)
			defer cnt1.close()

			time.Sleep(1 * time.Second)

			pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
			require.NoError(t, err)
			defer pc.Close()

			cnt2, err := newContainer("gstreamer", "source", []string{
				"filesrc location=emptyvideo.ts ! tsdemux ! video/x-h264" +
					" ! h264parse config-interval=1 ! rtph264pay ! udpsink host=127.0.0.1 port=" + strconv.FormatInt(int64(pc.LocalAddr().(*net.UDPAddr).Port), 10),
			})
			require.NoError(t, err)
			defer cnt2.close()

			decoder := rtph264.NewDecoder()
			sps, pps, err := decoder.ReadSPSPPS(rtph264.PacketConnReader{pc}) //nolint:govet
			require.NoError(t, err)

			track, err := NewTrackH264(96, sps, pps)
			require.NoError(t, err)

			conf := ClientConf{
				StreamProtocol: func() *StreamProtocol {
					if proto == "udp" {
						v := StreamProtocolUDP
						return &v
					}
					v := StreamProtocolTCP
					return &v
				}(),
			}

			conn, err := conf.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			writerDone := make(chan struct{})
			go func() {
				defer close(writerDone)

				buf := make([]byte, 2048)
				for {
					n, _, err := pc.ReadFrom(buf)
					require.NoError(t, err)

					err = conn.WriteFrame(track.ID, StreamTypeRTP, buf[:n])
					if err != nil {
						break
					}
				}
			}()

			time.Sleep(1 * time.Second)

			_, err = conn.Pause()
			require.NoError(t, err)
			<-writerDone

			conn.Close()
		})
	}
}
