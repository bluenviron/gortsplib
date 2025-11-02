//go:build enable_e2e_tests

package teste2e

import (
	"crypto/tls"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

	cmd := []string{
		"docker", "run",
		"--network=host",
		"--name=gortsplib-test-" + name,
		"gortsplib-test-" + image,
	}
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
	code, _ := strconv.ParseInt(string(out[:len(out)-1]), 10, 32)
	return int(code)
}

func buildImage(image string) error {
	ecmd := exec.Command("docker", "build", filepath.Join("images", image),
		"-t", "gortsplib-test-"+image)
	ecmd.Stdout = nil
	ecmd.Stderr = os.Stderr
	return ecmd.Run()
}

func TestServerVsExternal(t *testing.T) {
	files, err := os.ReadDir("images")
	require.NoError(t, err)

	var wg sync.WaitGroup

	for _, file := range files {
		cfile := file
		wg.Go(func() {
			err := buildImage(cfile.Name())
			require.NoError(t, err)
		})
	}

	wg.Wait()

	for _, ca := range []struct {
		publisherSoft    string
		publisherScheme  string
		publisherProto   string
		publisherProfile string
		publisherTunnel  string
		readerSoft       string
		readerScheme     string
		readerProto      string
		readerProfile    string
		readerTunnel     string
	}{
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "udp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "udp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "udp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsp",
			readerProto:      "udp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsp",
			publisherProto:   "udp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "udp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsp",
			publisherProto:   "udp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsp",
			readerProto:      "udp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "udp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "multicast",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "udp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsp",
			readerProto:      "multicast",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "udp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsp",
			publisherProto:   "udp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsps",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsps",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsps",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsps",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsps",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsps",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsps",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsps",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "ffmpeg",
			publisherScheme:  "rtsps",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsps",
			readerProto:      "udp",
			readerProfile:    "savp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsps",
			publisherProto:   "udp",
			publisherProfile: "savp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsps",
			readerProto:      "udp",
			readerProfile:    "savp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsps",
			publisherProto:   "udp",
			publisherProfile: "savp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsps",
			readerProto:      "multicast",
			readerProfile:    "savp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "http",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "none",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "ffmpeg",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "http",
		},
		{
			publisherSoft:    "gstreamer",
			publisherScheme:  "rtsp",
			publisherProto:   "tcp",
			publisherProfile: "avp",
			publisherTunnel:  "none",
			readerSoft:       "gstreamer",
			readerScheme:     "rtsp",
			readerProto:      "tcp",
			readerProfile:    "avp",
			readerTunnel:     "http",
		},
	} {
		t.Run(strings.Join([]string{
			ca.publisherSoft,
			ca.publisherScheme,
			ca.publisherProto,
			ca.publisherProfile,
			ca.publisherTunnel,
			ca.readerSoft,
			ca.readerScheme,
			ca.readerProto,
			ca.readerProfile,
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

			switch ca.publisherSoft {
			case "ffmpeg":
				cnt1, err := newContainer("ffmpeg", "publish", []string{
					"-re",
					"-stream_loop", "-1",
					"-i", "emptyvideo.mkv",
					"-c", "copy",
					"-f", "rtsp",
					"-rtsp_transport", ca.publisherProto,
					ca.publisherScheme + "://localhost:8554/test/stream?key=val",
				})
				require.NoError(t, err)
				defer cnt1.close()

			case "gstreamer":
				var scheme string
				if ca.publisherTunnel == "http" {
					scheme = "rtsph"
				} else {
					scheme = ca.publisherScheme
				}

				var profile string
				if ca.publisherProfile == "savp" {
					profile = "GST_RTSP_PROFILE_SAVP"
				} else {
					profile = "GST_RTSP_PROFILE_AVP"
				}

				cnt1, err := newContainer("gstreamer", "publish", []string{
					"filesrc location=emptyvideo.mkv ! matroskademux ! video/x-h264 ! rtspclientsink " +
						"location=" + scheme + "://127.0.0.1:8554/test/stream?key=val" +
						" protocols=" + ca.publisherProto +
						" profiles=" + profile +
						" tls-validation-flags=0 latency=0 timeout=0 rtx-time=0",
				})
				require.NoError(t, err)
				defer cnt1.close()

				time.Sleep(1 * time.Second)
			}

			time.Sleep(1 * time.Second)

			switch ca.readerSoft {
			case "ffmpeg":
				var proto string
				switch {
				case ca.readerTunnel == "http":
					proto = "http"

				case ca.readerProto == "multicast":
					proto = "udp_multicast"

				default:
					proto = ca.readerProto
				}

				cnt2, err := newContainer("ffmpeg", "read", []string{
					"-rtsp_transport", proto,
					"-i", ca.readerScheme + "://localhost:8554/test/stream?key=val",
					"-vframes", "1",
					"-f", "image2",
					"-y", "/dev/null",
				})
				require.NoError(t, err)
				defer cnt2.close()
				require.Equal(t, 0, cnt2.wait())

			case "gstreamer":
				var scheme string
				if ca.readerTunnel == "http" {
					scheme = "rtsph"
				} else {
					scheme = ca.readerScheme
				}

				var proto string
				if ca.readerProto == "multicast" {
					proto = "udp-mcast"
				} else {
					proto = ca.readerProto
				}

				cnt2, err := newContainer("gstreamer", "read", []string{
					"rtspsrc location=" + scheme + "://127.0.0.1:8554/test/stream?key=val" +
						" protocols=" + proto +
						" tls-validation-flags=0 latency=0 " +
						"! application/x-rtp,media=video ! decodebin ! video/x-raw ! fakesink num-buffers=1",
				})
				require.NoError(t, err)
				defer cnt2.close()
				require.Equal(t, 0, cnt2.wait())
			}
		})
	}
}
