package gortsplib

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/rtph264"
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

func TestConnClientDialReadUDP(t *testing.T) {
	cnt1, err := newContainer("rtsp-simple-server", "server", []string{})
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

	conn, _, err := DialRead("rtsp://localhost:8554/teststream", StreamProtocolUDP)
	require.NoError(t, err)
	defer conn.Close()

	loopDone := make(chan struct{})
	defer func() { <-loopDone }()

	go func() {
		defer close(loopDone)
		conn.LoopUDP()
	}()

	_, err = conn.ReadFrameUDP(0, StreamTypeRtp)
	require.NoError(t, err)

	conn.CloseUDPListeners()
}

func TestConnClientDialReadTCP(t *testing.T) {
	cnt1, err := newContainer("rtsp-simple-server", "server", []string{})
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

	conn, _, err := DialRead("rtsp://localhost:8554/teststream", StreamProtocolTCP)
	require.NoError(t, err)
	defer conn.Close()

	id, typ, _, err := conn.ReadFrameTCP()
	require.NoError(t, err)

	require.Equal(t, 0, id)
	require.Equal(t, StreamTypeRtp, typ)
}

func TestConnClientDialPublishUDP(t *testing.T) {
	cnt1, err := newContainer("rtsp-simple-server", "server", []string{})
	require.NoError(t, err)
	defer cnt1.close()

	time.Sleep(1 * time.Second)

	publishDone := make(chan struct{})
	defer func() { <-publishDone }()

	var conn *ConnClient
	defer func() {
		conn.Close()
	}()

	go func() {
		defer close(publishDone)

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

		conn, err = DialPublish("rtsp://localhost:8554/teststream",
			StreamProtocolUDP, Tracks{track})
		require.NoError(t, err)

		buf := make([]byte, 2048)
		for {
			n, _, err := pc.ReadFrom(buf)
			if err != nil {
				break
			}

			err = conn.WriteFrameUDP(track.Id, StreamTypeRtp, buf[:n])
			if err != nil {
				break
			}
		}
	}()

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
}

func TestConnClientDialPublishTCP(t *testing.T) {
	cnt1, err := newContainer("rtsp-simple-server", "server", []string{})
	require.NoError(t, err)
	defer cnt1.close()

	time.Sleep(1 * time.Second)

	publishDone := make(chan struct{})
	defer func() { <-publishDone }()

	var conn *ConnClient
	defer func() {
		conn.Close()
	}()

	go func() {
		defer close(publishDone)

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

		conn, err = DialPublish("rtsp://localhost:8554/teststream",
			StreamProtocolTCP, Tracks{track})
		require.NoError(t, err)

		buf := make([]byte, 2048)
		for {
			n, _, err := pc.ReadFrom(buf)
			if err != nil {
				break
			}

			err = conn.WriteFrameTCP(track.Id, StreamTypeRtp, buf[:n])
			if err != nil {
				break
			}
		}
	}()

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
}
