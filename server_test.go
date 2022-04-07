package gortsplib

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/auth"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

func writeReqReadRes(conn net.Conn,
	br *bufio.Reader,
	req base.Request,
) (*base.Response, error) {
	var bb bytes.Buffer
	req.Write(&bb)
	_, err := conn.Write(bb.Bytes())
	if err != nil {
		return nil, err
	}

	var res base.Response
	err = res.Read(br)
	return &res, err
}

func readResIgnoreFrames(br *bufio.Reader) (*base.Response, error) {
	buf := make([]byte, 2048)
	var res base.Response
	err := res.ReadIgnoreFrames(br, buf)
	return &res, err
}

type testServerHandler struct {
	onConnOpen     func(*ServerHandlerOnConnOpenCtx)
	onConnClose    func(*ServerHandlerOnConnCloseCtx)
	onSessionOpen  func(*ServerHandlerOnSessionOpenCtx)
	onSessionClose func(*ServerHandlerOnSessionCloseCtx)
	onDescribe     func(*ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error)
	onAnnounce     func(*ServerHandlerOnAnnounceCtx) (*base.Response, error)
	onSetup        func(*ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error)
	onPlay         func(*ServerHandlerOnPlayCtx) (*base.Response, error)
	onRecord       func(*ServerHandlerOnRecordCtx) (*base.Response, error)
	onPause        func(*ServerHandlerOnPauseCtx) (*base.Response, error)
	onPacketRTP    func(*ServerHandlerOnPacketRTPCtx)
	onPacketRTCP   func(*ServerHandlerOnPacketRTCPCtx)
	onSetParameter func(*ServerHandlerOnSetParameterCtx) (*base.Response, error)
	onGetParameter func(*ServerHandlerOnGetParameterCtx) (*base.Response, error)
}

func (sh *testServerHandler) OnConnOpen(ctx *ServerHandlerOnConnOpenCtx) {
	if sh.onConnOpen != nil {
		sh.onConnOpen(ctx)
	}
}

func (sh *testServerHandler) OnConnClose(ctx *ServerHandlerOnConnCloseCtx) {
	if sh.onConnClose != nil {
		sh.onConnClose(ctx)
	}
}

func (sh *testServerHandler) OnSessionOpen(ctx *ServerHandlerOnSessionOpenCtx) {
	if sh.onSessionOpen != nil {
		sh.onSessionOpen(ctx)
	}
}

func (sh *testServerHandler) OnSessionClose(ctx *ServerHandlerOnSessionCloseCtx) {
	if sh.onSessionClose != nil {
		sh.onSessionClose(ctx)
	}
}

func (sh *testServerHandler) OnDescribe(ctx *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
	if sh.onDescribe != nil {
		return sh.onDescribe(ctx)
	}
	return nil, nil, fmt.Errorf("unimplemented")
}

func (sh *testServerHandler) OnAnnounce(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	if sh.onAnnounce != nil {
		return sh.onAnnounce(ctx)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (sh *testServerHandler) OnSetup(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
	if sh.onSetup != nil {
		return sh.onSetup(ctx)
	}
	return nil, nil, fmt.Errorf("unimplemented")
}

func (sh *testServerHandler) OnPlay(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
	if sh.onPlay != nil {
		return sh.onPlay(ctx)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (sh *testServerHandler) OnRecord(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
	if sh.onRecord != nil {
		return sh.onRecord(ctx)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (sh *testServerHandler) OnPause(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
	if sh.onPause != nil {
		return sh.onPause(ctx)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (sh *testServerHandler) OnPacketRTP(ctx *ServerHandlerOnPacketRTPCtx) {
	if sh.onPacketRTP != nil {
		sh.onPacketRTP(ctx)
	}
}

func (sh *testServerHandler) OnPacketRTCP(ctx *ServerHandlerOnPacketRTCPCtx) {
	if sh.onPacketRTCP != nil {
		sh.onPacketRTCP(ctx)
	}
}

func (sh *testServerHandler) OnSetParameter(ctx *ServerHandlerOnSetParameterCtx) (*base.Response, error) {
	if sh.onSetParameter != nil {
		return sh.onSetParameter(ctx)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (sh *testServerHandler) OnGetParameter(ctx *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
	if sh.onGetParameter != nil {
		return sh.onGetParameter(ctx)
	}
	return nil, fmt.Errorf("unimplemented")
}

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
	code, _ := strconv.ParseInt(string(out[:len(out)-1]), 10, 64)
	return int(code)
}

var serverCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDkzCCAnugAwIBAgIUHFnymlrkEnz3ThpFvSrqybBepn4wDQYJKoZIhvcNAQEL
BQAwWTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDESMBAGA1UEAwwJbG9jYWxob3N0MB4X
DTIxMTIwMzIxNDg0MFoXDTMxMTIwMTIxNDg0MFowWTELMAkGA1UEBhMCQVUxEzAR
BgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoMGEludGVybmV0IFdpZGdpdHMgUHR5
IEx0ZDESMBAGA1UEAwwJbG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEAv8h21YDIAYNzewrfQqQTlODJjuUZKxMCO7z1wIapem5I+1I8n+vD
v8qvuyZk1m9CKQPfXxhJz0TT5kECoUY0KaDtykSzfaUK34F9J1d5snDkaOtN48W+
8l39Wtcvc5JW17jNwabppAkHHYAMQryO8urKLWKbZmLhYCJdYgNqb8ciWPsnYNA0
zcnKML9zQphh7dxPq1wCsy/c/XZUzxTLAe8hsCKuqpESEX3MMJA9gOLmiOF0JgpT
9h6eqvJU8IK0QMIv3tekJWSBvTLyz4ghENs10sMKKNqR6NWt2SsOloeBkOhIDLOk
byLaPEvugrQsga99uhANRpXp+CHnVeAH8QIDAQABo1MwUTAdBgNVHQ4EFgQUwyEH
cMynEoy1/TnbIhgpEAs038gwHwYDVR0jBBgwFoAUwyEHcMynEoy1/TnbIhgpEAs0
38gwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAiV56KhDoUVzW
qV1X0QbfLaifimsN3Na3lUgmjcgyUe8rHj09pXuAD/AcQw/zwKzZ6dPtizBeNLN8
jV1dbJmR7DE3MDlndgMKTOKFsqzHjG9UTXkBGFUEM1shn2GE8XcvDF0AzKU82YjP
B0KswA1NoYTNP2PW4IhZRzv2M+fnmkvc8DSEZ+dxEMg3aJfe/WLPvYjDpFXLvuxl
YnerRQ04hFysh5eogPFpB4KyyPs6jGnQFmZCbFyk9pjKRbDPJc6FkDglkzTB6j3Q
TSfgNJswOiap13vQQKf5Vu7LTuyO4Wjfjr74QNqMLLNIgcC7n2jfQj1g5Xa0bnF5
G4tLrCLUUw==
-----END CERTIFICATE-----
`)

var serverKey = []byte(`-----BEGIN PRIVATE KEY-----
MIIEvwIBADANBgkqhkiG9w0BAQEFAASCBKkwggSlAgEAAoIBAQC/yHbVgMgBg3N7
Ct9CpBOU4MmO5RkrEwI7vPXAhql6bkj7Ujyf68O/yq+7JmTWb0IpA99fGEnPRNPm
QQKhRjQpoO3KRLN9pQrfgX0nV3mycORo603jxb7yXf1a1y9zklbXuM3BpumkCQcd
gAxCvI7y6sotYptmYuFgIl1iA2pvxyJY+ydg0DTNycowv3NCmGHt3E+rXAKzL9z9
dlTPFMsB7yGwIq6qkRIRfcwwkD2A4uaI4XQmClP2Hp6q8lTwgrRAwi/e16QlZIG9
MvLPiCEQ2zXSwwoo2pHo1a3ZKw6Wh4GQ6EgMs6RvIto8S+6CtCyBr326EA1Glen4
IedV4AfxAgMBAAECggEAOqcJSNSA1o2oJKo3i374iiCRJAWGw/ilRzXMBtxoOow9
/7av2czV6fMH+XmNf1M5bafEiaW49Q28rH+XWVFKJK0V7DVEm5l9EMveRcjn7B3A
jSHhiVZxxlfeYwjKd1L7AjB/pMjyTXuBVJFTrplSMpKB0I2GrzJwcOExpAcdZx98
K0s5pauJH9bE0kI3p585SGQaIjrz0LvAmf6cQ5HhKfahJdWNnKZ/S4Kdqe+JCgyd
NawREHhf3tU01Cd3DOgXn4+5V/Ts6XtqY1RuSvonNv3nyeiOpX8C4cHKD5u2sNOC
3J4xWrrs0W3e8IATgAys56teKbEufHTUx52wNhAbzQKBgQD56W0tPCuaKrsjxsvE
dNHdm/9aQrN1jCJxUcGaxCIioXSyDvpSKcgxQbEqHXRTtJt5/Kadz9omq4vFTVtl
5Gf+3Lrf3ZT82SvYHtlIMdBZLlKwk6MolEa0KGAuJBNJVRIOkm5YjV/3bJebeTIb
WrLEyNCOXFAh3KVzBPU8nJ1aTwKBgQDEdISg3UsSOLBa0BfoJ5FlqGepZSufYgqh
xAJn8EbopnlzfmHBZAhE2+Igh0xcHhQqHThc3OuLtAkWu6fUSLiSA+XjU9TWPpA1
C/325rhT23fxzYIlYFegR9BToxYhv14ufkcTXRfHRAhffk7K5A2nlJfldDZRmUh2
5KIjXQ0pvwKBgQCa7S6VgFu3cw4Ym8DuxUzlCTRADGGcWYdwoLJY84YF2fmx+L8N
+ID2qDbgWOooiipocUwJQTWIC4jWg6JJhFNEGCpxZbhbF3aqwFULAHadEq6IcL4R
Bfre7LjTYeHi8C4FgpmNo/b+N/+0jmmVs6BnheZkmq3CkDqxFz3AmYai2QKBgQC1
kzAmcoJ5U/YD6YO/Khsjx3QQSBb6mCZVf5HtuVIApCVqzuvRUACojEbDY+n61j4y
8pDum64FkKA557Xl6lTVeE7ZPtlgL7EfpnbT5kmGEDobPqPEofg7h0SQmRLSnEqT
VFmjFw7sOQA4Ksjuk7vfIOMHy9KMts0YPpdxcgbBhwKBgQCP8MeRPuhZ26/oIESr
I8ArLEaPebYmLXCT2ZTudGztoyYFxinRGHA4PdamSOKfB1li52wAaqgRA3cSqkUi
kabimVOvrOAWlnvznqXEHPNx6mbbKs08jh+uRRmrOmMrxAobpTqarL2Sdxb6afID
NkxNic7oHgsZpIkZ8HK+QjAAWA==
-----END PRIVATE KEY-----
`)

func TestServerHighLevelPublishRead(t *testing.T) {
	for _, ca := range []struct {
		publisherSoft  string
		publisherProto string
		readerSoft     string
		readerProto    string
	}{
		{"ffmpeg", "udp", "ffmpeg", "udp"},
		{"ffmpeg", "udp", "gstreamer", "udp"},
		{"gstreamer", "udp", "ffmpeg", "udp"},
		{"gstreamer", "udp", "gstreamer", "udp"},

		{"ffmpeg", "tcp", "ffmpeg", "tcp"},
		{"ffmpeg", "tcp", "gstreamer", "tcp"},
		{"gstreamer", "tcp", "ffmpeg", "tcp"},
		{"gstreamer", "tcp", "gstreamer", "tcp"},

		{"ffmpeg", "tcp", "ffmpeg", "udp"},
		{"ffmpeg", "udp", "ffmpeg", "tcp"},

		{"ffmpeg", "tls", "ffmpeg", "tls"},
		{"ffmpeg", "tls", "gstreamer", "tls"},
		{"gstreamer", "tls", "ffmpeg", "tls"},
		{"gstreamer", "tls", "gstreamer", "tls"},

		{"ffmpeg", "udp", "ffmpeg", "multicast"},
		{"ffmpeg", "udp", "gstreamer", "multicast"},
	} {
		t.Run(ca.publisherSoft+"_"+ca.publisherProto+"_"+
			ca.readerSoft+"_"+ca.readerProto, func(t *testing.T) {
			var mutex sync.Mutex
			var stream *ServerStream
			var publisher *ServerSession

			s := &Server{
				Handler: &testServerHandler{
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						mutex.Lock()
						defer mutex.Unlock()

						if stream != nil {
							if ctx.Session == publisher {
								stream.Close()
								stream = nil
							}
						}
					},
					onDescribe: func(ctx *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						if ctx.Path != "test/stream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, nil, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
						}
						if ctx.Query != "key=val" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, nil, fmt.Errorf("invalid query (%s)", ctx.Query)
						}

						mutex.Lock()
						defer mutex.Unlock()

						if stream == nil {
							return &base.Response{
								StatusCode: base.StatusNotFound,
							}, nil, nil
						}

						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						if ctx.Path != "test/stream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
						}
						if ctx.Query != "key=val" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid query (%s)", ctx.Query)
						}

						mutex.Lock()
						defer mutex.Unlock()

						if stream != nil {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("someone is already publishing")
						}

						stream = NewServerStream(ctx.Tracks)
						publisher = ctx.Session

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						if ctx.Path != "test/stream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, nil, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
						}
						if ctx.Query != "key=val" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, nil, fmt.Errorf("invalid query (%s)", ctx.Query)
						}

						if stream == nil {
							return &base.Response{
								StatusCode: base.StatusNotFound,
							}, nil, nil
						}

						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						if ctx.Path != "test/stream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
						}
						if ctx.Query != "key=val" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid query (%s)", ctx.Query)
						}

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						if ctx.Path != "test/stream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
						}
						if ctx.Query != "key=val" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid query (%s)", ctx.Query)
						}

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onPacketRTP: func(ctx *ServerHandlerOnPacketRTPCtx) {
						mutex.Lock()
						defer mutex.Unlock()

						if ctx.Session == publisher {
							stream.WritePacketRTP(ctx.TrackID, ctx.Packet, true)
						}
					},
				},
				RTSPAddress: "localhost:8554",
			}

			var proto string
			if ca.publisherProto == "tls" {
				proto = "rtsps"
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
			} else {
				proto = "rtsp"
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"
				s.MulticastIPRange = "224.1.0.0/16"
				s.MulticastRTPPort = 8002
				s.MulticastRTCPPort = 8003
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			switch ca.publisherSoft {
			case "ffmpeg":
				ts := func() string {
					switch ca.publisherProto {
					case "udp", "tcp":
						return ca.publisherProto
					}
					return "tcp"
				}()

				cnt1, err := newContainer("ffmpeg", "publish", []string{
					"-re",
					"-stream_loop", "-1",
					"-i", "emptyvideo.mkv",
					"-c", "copy",
					"-f", "rtsp",
					"-rtsp_transport", ts,
					proto + "://localhost:8554/test/stream?key=val",
				})
				require.NoError(t, err)
				defer cnt1.close()

			case "gstreamer":
				ts := func() string {
					switch ca.publisherProto {
					case "udp", "tcp":
						return ca.publisherProto
					}
					return "tcp"
				}()

				cnt1, err := newContainer("gstreamer", "publish", []string{
					"filesrc location=emptyvideo.mkv ! matroskademux ! video/x-h264 ! rtspclientsink " +
						"location=" + proto + "://127.0.0.1:8554/test/stream?key=val protocols=" + ts +
						" tls-validation-flags=0 latency=0 timeout=0 rtx-time=0",
				})
				require.NoError(t, err)
				defer cnt1.close()

				time.Sleep(1 * time.Second)
			}

			time.Sleep(1 * time.Second)

			switch ca.readerSoft {
			case "ffmpeg":
				ts := func() string {
					switch ca.readerProto {
					case "udp", "tcp":
						return ca.readerProto
					case "multicast":
						return "udp_multicast"
					}
					return "tcp"
				}()

				cnt2, err := newContainer("ffmpeg", "read", []string{
					"-rtsp_transport", ts,
					"-i", proto + "://localhost:8554/test/stream?key=val",
					"-vframes", "1",
					"-f", "image2",
					"-y", "/dev/null",
				})
				require.NoError(t, err)
				defer cnt2.close()
				require.Equal(t, 0, cnt2.wait())

			case "gstreamer":
				ts := func() string {
					switch ca.readerProto {
					case "udp", "tcp":
						return ca.readerProto
					case "multicast":
						return "udp-mcast"
					}
					return "tcp"
				}()

				cnt2, err := newContainer("gstreamer", "read", []string{
					"rtspsrc location=" + proto + "://127.0.0.1:8554/test/stream?key=val protocols=" + ts +
						" tls-validation-flags=0 latency=0 " +
						"! application/x-rtp,media=video ! decodebin ! exitafterframe ! fakesink",
				})
				require.NoError(t, err)
				defer cnt2.close()
				require.Equal(t, 0, cnt2.wait())
			}
		})
	}
}

func TestServerClose(t *testing.T) {
	s := &Server{
		Handler:     &testServerHandler{},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	s.Close()
	s.Close()
}

func TestServerErrorInvalidUDPPorts(t *testing.T) {
	t.Run("non consecutive", func(t *testing.T) {
		s := &Server{
			UDPRTPAddress:  "127.0.0.1:8006",
			UDPRTCPAddress: "127.0.0.1:8009",
			RTSPAddress:    "localhost:8554",
		}
		err := s.Start()
		require.Error(t, err)
	})

	t.Run("non even", func(t *testing.T) {
		s := &Server{
			UDPRTPAddress:  "127.0.0.1:8003",
			UDPRTCPAddress: "127.0.0.1:8004",
			RTSPAddress:    "localhost:8554",
		}
		err := s.Start()
		require.Error(t, err)
	})
}

func TestServerConnClose(t *testing.T) {
	connClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnOpen: func(ctx *ServerHandlerOnConnOpenCtx) {
				ctx.Conn.Close()
				ctx.Conn.Close()
			},
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				close(connClosed)
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()

	<-connClosed
}

func TestServerCSeq(t *testing.T) {
	s := &Server{
		RTSPAddress: "localhost:8554",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Options,
		URL:    mustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"5"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	require.Equal(t, base.HeaderValue{"5"}, res.Header["CSeq"])
}

func TestServerErrorCSeqMissing(t *testing.T) {
	connClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				require.EqualError(t, ctx.Error, "CSeq is missing")
				close(connClosed)
			},
		},
		RTSPAddress: "localhost:8554",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Options,
		URL:    mustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	<-connClosed
}

func TestServerErrorInvalidMethod(t *testing.T) {
	connClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				require.EqualError(t, ctx.Error, "unhandled request: INVALID rtsp://localhost:8554/")
				close(connClosed)
			},
		},
		RTSPAddress: "localhost:8554",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: "INVALID",
		URL:    mustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	<-connClosed
}

func TestServerErrorTCPTwoConnOneSession(t *testing.T) {
	track, err := NewTrackH264(96, []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}, nil)
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn1, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn1.Close()
	br1 := bufio.NewReader(conn1)

	res, err := writeReqReadRes(conn1, br1, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Read(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn1, br1, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	conn2, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn2.Close()
	br2 := bufio.NewReader(conn2)

	res, err = writeReqReadRes(conn2, br2, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)
}

func TestServerErrorTCPOneConnTwoSessions(t *testing.T) {
	track, err := NewTrackH264(96, []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}, nil)
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var sx headers.Session
	err = sx.Read(res.Header["Session"])
	require.NoError(t, err)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Play,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{sx.Session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)
}

func TestServerGetSetParameter(t *testing.T) {
	var params []byte

	s := &Server{
		Handler: &testServerHandler{
			onSetParameter: func(ctx *ServerHandlerOnSetParameterCtx) (*base.Response, error) {
				params = ctx.Request.Body
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onGetParameter: func(ctx *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
					Body:       params,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Options,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.SetParameter,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"12"},
		},
		Body: []byte("param1: 123456\r\n"),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(conn, br, base.Request{
		Method: base.GetParameter,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
		},
		Body: []byte("param1\r\n"),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
	require.Equal(t, []byte("param1: 123456\r\n"), res.Body)
}

func TestServerErrorInvalidSession(t *testing.T) {
	for _, method := range []base.Method{
		base.Play,
		base.Record,
		base.Pause,
		base.Teardown,
	} {
		t.Run(string(method), func(t *testing.T) {
			s := &Server{
				Handler: &testServerHandler{
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onPause: func(ctx *ServerHandlerOnPauseCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress: "localhost:8554",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			br := bufio.NewReader(conn)

			res, err := writeReqReadRes(conn, br, base.Request{
				Method: method,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"1"},
					"Session": base.HeaderValue{"ABC"},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusSessionNotFound, res.StatusCode)
		})
	}
}

func TestServerSessionClose(t *testing.T) {
	sessionClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onSessionOpen: func(ctx *ServerHandlerOnSessionOpenCtx) {
				ctx.Session.Close()
				ctx.Session.Close()
			},
			onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
				close(sessionClosed)
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	var bb bytes.Buffer

	base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	}.Write(&bb)
	_, err = conn.Write(bb.Bytes())
	require.NoError(t, err)

	<-sessionClosed
}

func TestServerSessionAutoClose(t *testing.T) {
	sessionClosed := make(chan struct{})

	track, err := NewTrackH264(96, []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}, nil)
	require.NoError(t, err)

	stream := NewServerStream(Tracks{track})
	defer stream.Close()

	s := &Server{
		Handler: &testServerHandler{
			onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
				close(sessionClosed)
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err = s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	br := bufio.NewReader(conn)

	res, err := writeReqReadRes(conn, br, base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: headers.TransportProtocolTCP,
				Delivery: func() *headers.TransportDelivery {
					v := headers.TransportDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	conn.Close()

	<-sessionClosed
}

func TestServerErrorInvalidPath(t *testing.T) {
	for _, method := range []base.Method{
		base.Describe,
		base.Announce,
		base.Play,
		base.Record,
		base.Pause,
		// base.GetParameter,
		// base.SetParameter,
	} {
		t.Run(string(method), func(t *testing.T) {
			connClosed := make(chan struct{})

			track, err := NewTrackH264(96, []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}, nil)
			require.NoError(t, err)

			stream := NewServerStream(Tracks{track})
			defer stream.Close()

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						require.EqualError(t, ctx.Error, "invalid path")
						close(connClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress: "localhost:8554",
			}

			err = s.Start()
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			br := bufio.NewReader(conn)

			sxID := ""

			if method == base.Record {
				track, err := NewTrackH264(96, []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}, nil)
				require.NoError(t, err)

				tracks := Tracks{track}
				tracks.setControls()

				res, err := writeReqReadRes(conn, br, base.Request{
					Method: base.Announce,
					URL:    mustParseURL("rtsp://localhost:8554/teststream"),
					Header: base.Header{
						"CSeq":         base.HeaderValue{"1"},
						"Content-Type": base.HeaderValue{"application/sdp"},
					},
					Body: tracks.Write(false),
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)
			}

			if method == base.Play || method == base.Record || method == base.Pause {
				res, err := writeReqReadRes(conn, br, base.Request{
					Method: base.Setup,
					URL:    mustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"2"},
						"Session": base.HeaderValue{sxID},
						"Transport": headers.Transport{
							Protocol: headers.TransportProtocolTCP,
							Delivery: func() *headers.TransportDelivery {
								v := headers.TransportDeliveryUnicast
								return &v
							}(),
							Mode: func() *headers.TransportMode {
								if method == base.Play || method == base.Pause {
									v := headers.TransportModePlay
									return &v
								}
								v := headers.TransportModeRecord
								return &v
							}(),
							InterleavedIDs: &[2]int{0, 1},
						}.Write(),
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)

				var sx headers.Session
				err = sx.Read(res.Header["Session"])
				require.NoError(t, err)

				sxID = sx.Session
			}

			if method == base.Pause {
				res, err := writeReqReadRes(conn, br, base.Request{
					Method: base.Play,
					URL:    mustParseURL("rtsp://localhost:8554/teststream/"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"2"},
						"Session": base.HeaderValue{sxID},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)
			}

			res, err := writeReqReadRes(conn, br, base.Request{
				Method: method,
				URL:    mustParseURL("rtsp://localhost:8554"),
				Header: base.Header{
					"CSeq":    base.HeaderValue{"3"},
					"Session": base.HeaderValue{sxID},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusBadRequest, res.StatusCode)

			<-connClosed
		})
	}
}

func TestServerAuth(t *testing.T) {
	authValidator := auth.NewValidator("myuser", "mypass", nil)

	s := &Server{
		Handler: &testServerHandler{
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				err := authValidator.ValidateRequest(ctx.Request)
				if err != nil {
					return &base.Response{
						StatusCode: base.StatusUnauthorized,
						Header: base.Header{
							"WWW-Authenticate": authValidator.Header(),
						},
					}, nil
				}

				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	br := bufio.NewReader(conn)

	track, err := NewTrackH264(96, []byte{0x01, 0x02, 0x03, 0x04}, []byte{0x01, 0x02, 0x03, 0x04}, nil)
	require.NoError(t, err)

	req := base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: Tracks{track}.Write(false),
	}

	res, err := writeReqReadRes(conn, br, req)
	require.NoError(t, err)
	require.Equal(t, base.StatusUnauthorized, res.StatusCode)

	sender, err := auth.NewSender(res.Header["WWW-Authenticate"], "myuser", "mypass")
	require.NoError(t, err)

	sender.AddAuthorization(&req)
	res, err = writeReqReadRes(conn, br, req)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}
