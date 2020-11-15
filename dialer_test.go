package gortsplib

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/rtph264"
)

type container struct {
	name string
}

func newContainer(image string, name string, args []string) (*container, error) {
	c := &container{
		name: name,
	}

	exec.Command("docker", "kill", "gortsplib-test-"+name).Run()
	exec.Command("docker", "wait", "gortsplib-test-"+name).Run()

	cmd := []string{"docker", "run",
		"--network=host",
		"--name=gortsplib-test-" + name,
		"gortsplib-test-" + image}
	cmd = append(cmd, args...)
	ecmd := exec.Command(cmd[0], cmd[1:]...)
	ecmd.Stdout = nil
	ecmd.Stderr = os.Stderr

	err := ecmd.Start()
	if err != nil {
		return nil, err
	}

	time.Sleep(1 * time.Second)

	return c, nil
}

func (c *container) close() {
	exec.Command("docker", "kill", "gortsplib-test-"+c.name).Run()
	exec.Command("docker", "wait", "gortsplib-test-"+c.name).Run()
	exec.Command("docker", "rm", "gortsplib-test-"+c.name).Run()
}

func (c *container) wait() int {
	exec.Command("docker", "wait", "gortsplib-test-"+c.name).Run()
	out, _ := exec.Command("docker", "inspect", "gortsplib-test-"+c.name,
		"--format={{.State.ExitCode}}").Output()
	code, _ := strconv.ParseInt(string(out[:len(out)-1]), 10, 64)
	return int(code)
}

func TestDialRead(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
			require.NoError(t, err)
			defer cnt1.close()

			time.Sleep(1 * time.Second)

			cnt2, err := newContainer("ffmpeg", "publish", []string{
				"-re",
				"-stream_loop", "-1",
				"-i", "/emptyvideo.ts",
				"-c", "copy",
				"-f", "rtsp",
				"-rtsp_transport", "udp",
				"rtsp://localhost:8554/teststream",
			})
			require.NoError(t, err)
			defer cnt2.close()

			time.Sleep(1 * time.Second)

			dialer := func() Dialer {
				if proto == "udp" {
					return Dialer{}
				}
				return Dialer{StreamProtocol: StreamProtocolTCP}
			}()

			conn, err := dialer.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			id, typ, _, err := conn.ReadFrame()
			require.NoError(t, err)

			require.Equal(t, 0, id)
			require.Equal(t, StreamTypeRtp, typ)

			conn.Close()

			_, _, _, err = conn.ReadFrame()
			require.Error(t, err)
		})
	}
}

func TestDialReadClose(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
			require.NoError(t, err)
			defer cnt1.close()

			time.Sleep(1 * time.Second)

			cnt2, err := newContainer("ffmpeg", "publish", []string{
				"-re",
				"-stream_loop", "-1",
				"-i", "/emptyvideo.ts",
				"-c", "copy",
				"-f", "rtsp",
				"-rtsp_transport", "udp",
				"rtsp://localhost:8554/teststream",
			})
			require.NoError(t, err)
			defer cnt2.close()

			time.Sleep(1 * time.Second)

			dialer := func() Dialer {
				if proto == "udp" {
					return Dialer{}
				}
				return Dialer{StreamProtocol: StreamProtocolTCP}
			}()

			conn, err := dialer.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			readDone := make(chan struct{})
			go func() {
				defer close(readDone)

				for {
					_, _, _, err := conn.ReadFrame()
					if err != nil {
						break
					}
				}
			}()

			time.Sleep(1 * time.Second)

			conn.Close()
			<-readDone
		})
	}
}

func TestDialReadRedirect(t *testing.T) {
	cnt1, err := newContainer("rtsp-simple-server", "server", []string{
		"paths:\n" +
			"  path1:\n" +
			"    source: redirect\n" +
			"    sourceRedirect: rtsp://localhost:8554/path2\n" +
			"  path2:\n",
	})
	require.NoError(t, err)
	defer cnt1.close()

	time.Sleep(1 * time.Second)

	cnt2, err := newContainer("ffmpeg", "publish", []string{
		"-re",
		"-stream_loop", "-1",
		"-i", "/emptyvideo.ts",
		"-c", "copy",
		"-f", "rtsp",
		"-rtsp_transport", "udp",
		"rtsp://localhost:8554/path2",
	})
	require.NoError(t, err)
	defer cnt2.close()

	time.Sleep(1 * time.Second)

	conn, err := DialRead("rtsp://localhost:8554/path1")
	require.NoError(t, err)
	defer conn.Close()

	_, _, _, err = conn.ReadFrame()
	require.NoError(t, err)
}

func TestDialReadPause(t *testing.T) {
	for _, proto := range []string{
		"udp",
		"tcp",
	} {
		t.Run(proto, func(t *testing.T) {
			cnt1, err := newContainer("rtsp-simple-server", "server", []string{"{}"})
			require.NoError(t, err)
			defer cnt1.close()

			time.Sleep(1 * time.Second)

			cnt2, err := newContainer("ffmpeg", "publish", []string{
				"-re",
				"-stream_loop", "-1",
				"-i", "/emptyvideo.ts",
				"-c", "copy",
				"-f", "rtsp",
				"-rtsp_transport", "udp",
				"rtsp://localhost:8554/teststream",
			})
			require.NoError(t, err)
			defer cnt2.close()

			time.Sleep(1 * time.Second)

			dialer := func() Dialer {
				if proto == "udp" {
					return Dialer{}
				}
				return Dialer{StreamProtocol: StreamProtocolTCP}
			}()

			conn, err := dialer.DialRead("rtsp://localhost:8554/teststream")
			require.NoError(t, err)
			defer conn.Close()

			_, _, _, err = conn.ReadFrame()
			require.NoError(t, err)

			_, err = conn.Pause()
			require.NoError(t, err)

			_, err = conn.Play()
			require.NoError(t, err)

			_, _, _, err = conn.ReadFrame()
			require.NoError(t, err)
		})
	}
}

func TestDialPublish(t *testing.T) {
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

			decoder := rtph264.NewDecoderFromPacketConn(pc)
			sps, pps, err := decoder.ReadSPSPPS()
			require.NoError(t, err)

			track, err := NewTrackH264(0, sps, pps)
			require.NoError(t, err)

			publishDone := make(chan struct{})
			defer func() { <-publishDone }()

			var conn *ConnClient
			defer func() { conn.Close() }()

			dialer := func() Dialer {
				if ca.proto == "udp" {
					return Dialer{}
				}
				return Dialer{StreamProtocol: StreamProtocolTCP}
			}()

			go func() {
				defer close(publishDone)

				port := "8554"
				if ca.server == "ffmpeg" {
					port = "8555"
				}
				var err error
				conn, err = dialer.DialPublish("rtsp://localhost:"+port+"/teststream",
					Tracks{track})
				require.NoError(t, err)

				buf := make([]byte, 2048)
				for {
					n, _, err := pc.ReadFrom(buf)
					if err != nil {
						break
					}

					err = conn.WriteFrame(track.Id, StreamTypeRtp, buf[:n])
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

func TestDialPublishClose(t *testing.T) {
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

			decoder := rtph264.NewDecoderFromPacketConn(pc)
			sps, pps, err := decoder.ReadSPSPPS()
			require.NoError(t, err)

			track, err := NewTrackH264(0, sps, pps)
			require.NoError(t, err)

			dialer := func() Dialer {
				if proto == "udp" {
					return Dialer{}
				}
				return Dialer{StreamProtocol: StreamProtocolTCP}
			}()

			conn, err := dialer.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)

			writeDone := make(chan struct{})
			go func() {
				defer close(writeDone)

				buf := make([]byte, 2048)
				for {
					n, _, err := pc.ReadFrom(buf)
					require.NoError(t, err)

					err = conn.WriteFrame(track.Id, StreamTypeRtp, buf[:n])
					if err != nil {
						break
					}
				}
			}()

			time.Sleep(1 * time.Second)

			conn.Close()
			<-writeDone
		})
	}
}

func TestDialPublishPause(t *testing.T) {
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

			decoder := rtph264.NewDecoderFromPacketConn(pc)
			sps, pps, err := decoder.ReadSPSPPS()
			require.NoError(t, err)

			track, err := NewTrackH264(0, sps, pps)
			require.NoError(t, err)

			dialer := func() Dialer {
				if proto == "udp" {
					return Dialer{}
				}
				return Dialer{StreamProtocol: StreamProtocolTCP}
			}()

			conn, err := dialer.DialPublish("rtsp://localhost:8554/teststream",
				Tracks{track})
			require.NoError(t, err)
			defer conn.Close()

			buf := make([]byte, 2048)

			n, _, err := pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.Id, StreamTypeRtp, buf[:n])
			require.NoError(t, err)

			_, err = conn.Pause()
			require.NoError(t, err)

			n, _, err = pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.Id, StreamTypeRtp, buf[:n])
			require.Error(t, err)

			_, err = conn.Record()
			require.NoError(t, err)

			n, _, err = pc.ReadFrom(buf)
			require.NoError(t, err)
			err = conn.WriteFrame(track.Id, StreamTypeRtp, buf[:n])
			require.NoError(t, err)
		})
	}
}
