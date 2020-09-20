package gortsplib

import (
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestConnClientTCP(t *testing.T) {
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

	u, err := url.Parse("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	conn, err := NewConnClient(ConnClientConf{Host: u.Host})
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	tracks, _, err := conn.Describe(u)
	require.NoError(t, err)

	for _, track := range tracks {
		_, err := conn.SetupTCP(u, track)
		require.NoError(t, err)
	}

	_, err = conn.Play(u)
	require.NoError(t, err)

	frame := &InterleavedFrame{Content: make([]byte, 0, 128*1024)}
	frame.Content = frame.Content[:cap(frame.Content)]

	err = conn.ReadFrame(frame)
	require.NoError(t, err)
}

func TestConnClientUDP(t *testing.T) {
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

	u, err := url.Parse("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	conn, err := NewConnClient(ConnClientConf{Host: u.Host})
	require.NoError(t, err)
	defer conn.Close()

	_, err = conn.Options(u)
	require.NoError(t, err)

	tracks, _, err := conn.Describe(u)
	require.NoError(t, err)

	var rtpReads []UDPReadFunc
	var rtcpReads []UDPReadFunc

	for _, track := range tracks {
		rtpRead, rtcpRead, _, err := conn.SetupUDP(u, track, 0, 0)
		require.NoError(t, err)

		rtpReads = append(rtpReads, rtpRead)
		rtcpReads = append(rtcpReads, rtcpRead)
	}

	_, err = conn.Play(u)
	require.NoError(t, err)

	go conn.LoopUDP(u)

	buf := make([]byte, 2048)
	_, err = rtpReads[0](buf)
	require.NoError(t, err)
}
