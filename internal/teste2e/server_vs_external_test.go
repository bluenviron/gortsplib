//go:build enable_e2e_tests

package teste2e

import (
	"crypto/tls"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
		wg.Add(1)
		go func() {
			err := buildImage(cfile.Name())
			require.NoError(t, err)
			wg.Done()
		}()
	}

	wg.Wait()

	for _, ca := range []struct {
		publisherSoft   string
		publisherScheme string
		publisherProto  string
		publisherSecure string
		readerSoft      string
		readerScheme    string
		readerProto     string
		readerSecure    string
	}{
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherSecure: "unsecure",
			readerSoft:      "gstreamer",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "gstreamer",
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "gstreamer",
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherSecure: "unsecure",
			readerSoft:      "gstreamer",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsp",
			readerProto:     "multicast",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherSecure: "unsecure",
			readerSoft:      "gstreamer",
			readerScheme:    "rtsp",
			readerProto:     "multicast",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "gstreamer",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "gstreamer",
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "gstreamer",
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "gstreamer",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsp",
			readerProto:     "udp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsp",
			publisherProto:  "udp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsp",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsps",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsps",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "ffmpeg",
			publisherScheme: "rtsps",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "gstreamer",
			readerScheme:    "rtsps",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
		{
			publisherSoft:   "gstreamer",
			publisherScheme: "rtsps",
			publisherProto:  "tcp",
			publisherSecure: "unsecure",
			readerSoft:      "ffmpeg",
			readerScheme:    "rtsps",
			readerProto:     "tcp",
			readerSecure:    "unsecure",
		},
	} {
		t.Run(ca.publisherSoft+"_"+ca.publisherScheme+"_"+ca.publisherProto+"_"+ca.publisherSecure+"_"+
			ca.readerSoft+"_"+ca.readerScheme+"_"+ca.readerProto+"_"+ca.readerSecure, func(t *testing.T) {
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
				var profile string
				if ca.publisherSecure == "secure" {
					profile = "GST_RTSP_PROFILE_SAVP"
				} else {
					profile = "GST_RTSP_PROFILE_AVP"
				}

				cnt1, err := newContainer("gstreamer", "publish", []string{
					"filesrc location=emptyvideo.mkv ! matroskademux ! video/x-h264 ! rtspclientsink " +
						"location=" + ca.publisherScheme + "://127.0.0.1:8554/test/stream?key=val" +
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
				if ca.readerProto == "multicast" {
					proto = "udp_multicast"
				} else {
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
				var proto string
				if ca.readerProto == "multicast" {
					proto = "udp-mcast"
				} else {
					proto = ca.readerProto
				}

				cnt2, err := newContainer("gstreamer", "read", []string{
					"rtspsrc location=" + ca.readerScheme + "://127.0.0.1:8554/test/stream?key=val" +
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
