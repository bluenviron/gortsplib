package gortsplib

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"
	"time"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

func writeReqReadRes(bconn *bufio.ReadWriter, req base.Request) (*base.Response, error) {
	err := req.Write(bconn.Writer)
	if err != nil {
		return nil, err
	}

	var res base.Response
	err = res.Read(bconn.Reader)
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
	onDescribe     func(*ServerHandlerOnDescribeCtx) (*base.Response, []byte, error)
	onAnnounce     func(*ServerHandlerOnAnnounceCtx) (*base.Response, error)
	onSetup        func(*ServerHandlerOnSetupCtx) (*base.Response, error)
	onPlay         func(*ServerHandlerOnPlayCtx) (*base.Response, error)
	onRecord       func(*ServerHandlerOnRecordCtx) (*base.Response, error)
	onPause        func(*ServerHandlerOnPauseCtx) (*base.Response, error)
	onFrame        func(*ServerHandlerOnFrameCtx)
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

func (sh *testServerHandler) OnDescribe(ctx *ServerHandlerOnDescribeCtx) (*base.Response, []byte, error) {
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

func (sh *testServerHandler) OnSetup(ctx *ServerHandlerOnSetupCtx) (*base.Response, error) {
	if sh.onSetup != nil {
		return sh.onSetup(ctx)
	}
	return nil, fmt.Errorf("unimplemented")
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

func (sh *testServerHandler) OnFrame(ctx *ServerHandlerOnFrameCtx) {
	if sh.onFrame != nil {
		sh.onFrame(ctx)
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

var serverCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDazCCAlOgAwIBAgIUXw1hEC3LFpTsllv7D3ARJyEq7sIwDQYJKoZIhvcNAQEL
BQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM
GEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMDEyMTMxNzQ0NThaFw0zMDEy
MTExNzQ0NThaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw
HwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDG8DyyS51810GsGwgWr5rjJK7OE1kTTLSNEEKax8Bj
zOyiaz8rA2JGl2VUEpi2UjDr9Cm7nd+YIEVs91IIBOb7LGqObBh1kGF3u5aZxLkv
NJE+HrLVvUhaDobK2NU+Wibqc/EI3DfUkt1rSINvv9flwTFu1qHeuLWhoySzDKEp
OzYxpFhwjVSokZIjT4Red3OtFz7gl2E6OAWe2qoh5CwLYVdMWtKR0Xuw3BkDPk9I
qkQKx3fqv97LPEzhyZYjDT5WvGrgZ1WDAN3booxXF3oA1H3GHQc4m/vcLatOtb8e
nI59gMQLEbnp08cl873bAuNuM95EZieXTHNbwUnq5iybAgMBAAGjUzBRMB0GA1Ud
DgQWBBQBKhJh8eWu0a4au9X/2fKhkFX2vjAfBgNVHSMEGDAWgBQBKhJh8eWu0a4a
u9X/2fKhkFX2vjAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQBj
3aCW0YPKukYgVK9cwN0IbVy/D0C1UPT4nupJcy/E0iC7MXPZ9D/SZxYQoAkdptdO
xfI+RXkpQZLdODNx9uvV+cHyZHZyjtE5ENu/i5Rer2cWI/mSLZm5lUQyx+0KZ2Yu
tEI1bsebDK30msa8QSTn0WidW9XhFnl3gRi4wRdimcQapOWYVs7ih+nAlSvng7NI
XpAyRs8PIEbpDDBMWnldrX4TP6EWYUi49gCp8OUDRREKX3l6Ls1vZ02F34yHIt/7
7IV/XSKG096bhW+icKBWV0IpcEsgTzPK1J1hMxgjhzIMxGboAeUU+kidthOob6Sd
XQxaORfgM//NzX9LhUPk
-----END CERTIFICATE-----
`)

var serverKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEogIBAAKCAQEAxvA8skudfNdBrBsIFq+a4ySuzhNZE0y0jRBCmsfAY8zsoms/
KwNiRpdlVBKYtlIw6/Qpu53fmCBFbPdSCATm+yxqjmwYdZBhd7uWmcS5LzSRPh6y
1b1IWg6GytjVPlom6nPxCNw31JLda0iDb7/X5cExbtah3ri1oaMkswyhKTs2MaRY
cI1UqJGSI0+EXndzrRc+4JdhOjgFntqqIeQsC2FXTFrSkdF7sNwZAz5PSKpECsd3
6r/eyzxM4cmWIw0+Vrxq4GdVgwDd26KMVxd6ANR9xh0HOJv73C2rTrW/HpyOfYDE
CxG56dPHJfO92wLjbjPeRGYnl0xzW8FJ6uYsmwIDAQABAoIBACi0BKcyQ3HElSJC
kaAao+Uvnzh4yvPg8Nwf5JDIp/uDdTMyIEWLtrLczRWrjGVZYbsVROinP5VfnPTT
kYwkfKINj2u+gC6lsNuPnRuvHXikF8eO/mYvCTur1zZvsQnF5kp4GGwIqr+qoPUP
bB0UMndG1PdpoMryHe+JcrvTrLHDmCeH10TqOwMsQMLHYLkowvxwJWsmTY7/Qr5S
Wm3PPpOcW2i0uyPVuyuv4yD1368fqnqJ8QFsQp1K6QtYsNnJ71Hut1/IoxK/e6hj
5Z+byKtHVtmcLnABuoOT7BhleJNFBksX9sh83jid4tMBgci+zXNeGmgqo2EmaWAb
agQslkECgYEA8B1rzjOHVQx/vwSzDa4XOrpoHQRfyElrGNz9JVBvnoC7AorezBXQ
M9WTHQIFTGMjzD8pb+YJGi3gj93VN51r0SmJRxBaBRh1ZZI9kFiFzngYev8POgD3
ygmlS3kTHCNxCK/CJkB+/jMBgtPj5ygDpCWVcTSuWlQFphePkW7jaaECgYEA1Blz
ulqgAyJHZaqgcbcCsI2q6m527hVr9pjzNjIVmkwu38yS9RTCgdlbEVVDnS0hoifl
+jVMEGXjF3xjyMvL50BKbQUH+KAa+V4n1WGlnZOxX9TMny8MBjEuSX2+362vQ3BX
4vOlX00gvoc+sY+lrzvfx/OdPCHQGVYzoKCxhLsCgYA07HcviuIAV/HsO2/vyvhp
xF5gTu+BqNUHNOZDDDid+ge+Jre2yfQLCL8VPLXIQW3Jff53IH/PGl+NtjphuLvj
7UDJvgvpZZuymIojP6+2c3gJ3CASC9aR3JBnUzdoE1O9s2eaoMqc4scpe+SWtZYf
3vzSZ+cqF6zrD/Rf/M35IQKBgHTU4E6ShPm09CcoaeC5sp2WK8OevZw/6IyZi78a
r5Oiy18zzO97U/k6xVMy6F+38ILl/2Rn31JZDVJujniY6eSkIVsUHmPxrWoXV1HO
y++U32uuSFiXDcSLarfIsE992MEJLSAynbF1Rsgsr3gXbGiuToJRyxbIeVy7gwzD
94TpAoGAY4/PejWQj9psZfAhyk5dRGra++gYRQ/gK1IIc1g+Dd2/BxbT/RHr05GK
6vwrfjsoRyMWteC1SsNs/CurjfQ/jqCfHNP5XPvxgd5Ec8sRJIiV7V5RTuWJsPu1
+3K6cnKEyg+0ekYmLertRFIY6SwWmY1fyKgTvxudMcsBY7dC4xs=
-----END RSA PRIVATE KEY-----
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
	} {
		t.Run(ca.publisherSoft+"_"+ca.publisherProto+"_"+
			ca.readerSoft+"_"+ca.readerProto, func(t *testing.T) {
			var mutex sync.Mutex
			var publisher *ServerSession
			var sdp []byte
			readers := make(map[*ServerSession]struct{})

			s := &Server{
				Handler: &testServerHandler{
					onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
						mutex.Lock()
						defer mutex.Unlock()

						if ctx.Session == publisher {
							publisher = nil
							sdp = nil
						} else {
							delete(readers, ctx.Session)
						}
					},
					onDescribe: func(ctx *ServerHandlerOnDescribeCtx) (*base.Response, []byte, error) {
						if ctx.Path != "teststream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, nil, fmt.Errorf("invalid path (%s)", ctx.Req.URL)
						}

						mutex.Lock()
						defer mutex.Unlock()

						if publisher == nil {
							return &base.Response{
								StatusCode: base.StatusNotFound,
							}, nil, nil
						}

						return &base.Response{
							StatusCode: base.StatusOK,
						}, sdp, nil
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						if ctx.Path != "teststream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid path (%s)", ctx.Req.URL)
						}

						mutex.Lock()
						defer mutex.Unlock()

						if publisher != nil {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("someone is already publishing")
						}

						publisher = ctx.Session
						sdp = ctx.Tracks.Write()

						return &base.Response{
							StatusCode: base.StatusOK,
							Header: base.Header{
								"Session": base.HeaderValue{"12345678"},
							},
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, error) {
						if ctx.Path != "teststream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid path (%s)", ctx.Req.URL)
						}

						return &base.Response{
							StatusCode: base.StatusOK,
							Header: base.Header{
								"Session": base.HeaderValue{"12345678"},
							},
						}, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						if ctx.Path != "teststream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid path (%s)", ctx.Req.URL)
						}

						mutex.Lock()
						defer mutex.Unlock()

						readers[ctx.Session] = struct{}{}

						return &base.Response{
							StatusCode: base.StatusOK,
							Header: base.Header{
								"Session": base.HeaderValue{"12345678"},
							},
						}, nil
					},
					onRecord: func(ctx *ServerHandlerOnRecordCtx) (*base.Response, error) {
						if ctx.Path != "teststream" {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("invalid path (%s)", ctx.Req.URL)
						}

						mutex.Lock()
						defer mutex.Unlock()

						if ctx.Session != publisher {
							return &base.Response{
								StatusCode: base.StatusBadRequest,
							}, fmt.Errorf("someone is already publishing")
						}

						return &base.Response{
							StatusCode: base.StatusOK,
							Header: base.Header{
								"Session": base.HeaderValue{"12345678"},
							},
						}, nil
					},
					onFrame: func(ctx *ServerHandlerOnFrameCtx) {
						mutex.Lock()
						defer mutex.Unlock()

						if ctx.Session == publisher {
							for r := range readers {
								r.WriteFrame(ctx.TrackID, ctx.StreamType, ctx.Payload)
							}
						}
					},
				},
			}

			var proto string
			var publisherSubProto string
			var readerSubProto string
			if ca.publisherProto == "tls" {
				proto = "rtsps"
				publisherSubProto = "tcp"
				readerSubProto = "tcp"
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				s.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}

			} else {
				proto = "rtsp"
				publisherSubProto = ca.publisherProto
				readerSubProto = ca.readerProto
				s.UDPRTPAddress = "127.0.0.1:8000"
				s.UDPRTCPAddress = "127.0.0.1:8001"
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			switch ca.publisherSoft {
			case "ffmpeg":
				cnt1, err := newContainer("ffmpeg", "publish", []string{
					"-re",
					"-stream_loop", "-1",
					"-i", "emptyvideo.mkv",
					"-c", "copy",
					"-f", "rtsp",
					"-rtsp_transport", publisherSubProto,
					proto + "://localhost:8554/teststream",
				})
				require.NoError(t, err)
				defer cnt1.close()

			case "gstreamer":
				cnt1, err := newContainer("gstreamer", "publish", []string{
					"filesrc location=emptyvideo.mkv ! matroskademux ! video/x-h264 ! rtspclientsink " +
						"location=" + proto + "://127.0.0.1:8554/teststream protocols=" + publisherSubProto + " tls-validation-flags=0 latency=0 timeout=0 rtx-time=0",
				})
				require.NoError(t, err)
				defer cnt1.close()

				time.Sleep(1 * time.Second)
			}

			time.Sleep(1 * time.Second)

			switch ca.readerSoft {
			case "ffmpeg":
				cnt2, err := newContainer("ffmpeg", "read", []string{
					"-rtsp_transport", readerSubProto,
					"-i", proto + "://localhost:8554/teststream",
					"-vframes", "1",
					"-f", "image2",
					"-y", "/dev/null",
				})
				require.NoError(t, err)
				defer cnt2.close()
				require.Equal(t, 0, cnt2.wait())

			case "gstreamer":
				cnt2, err := newContainer("gstreamer", "read", []string{
					"rtspsrc location=" + proto + "://127.0.0.1:8554/teststream protocols=" + readerSubProto + " tls-validation-flags=0 latency=0 " +
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
		Handler: &testServerHandler{},
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	s.Close()
	s.Close()
}

func TestServerErrorWrongUDPPorts(t *testing.T) {
	t.Run("non consecutive", func(t *testing.T) {
		s := &Server{
			UDPRTPAddress:  "127.0.0.1:8006",
			UDPRTCPAddress: "127.0.0.1:8009",
		}
		err := s.Start("127.0.0.1:8554")
		require.Error(t, err)
	})

	t.Run("non even", func(t *testing.T) {
		s := &Server{
			UDPRTPAddress:  "127.0.0.1:8003",
			UDPRTCPAddress: "127.0.0.1:8004",
		}
		err := s.Start("127.0.0.1:8554")
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
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()

	<-connClosed
}

func TestServerCSeq(t *testing.T) {
	s := &Server{}
	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Options,
		URL:    base.MustParseURL("rtsp://localhost:8554/"),
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

	h := &testServerHandler{
		onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
			require.Equal(t, "CSeq is missing", ctx.Error.Error())
			close(connClosed)
		},
	}

	s := &Server{Handler: h}
	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Options,
		URL:    base.MustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	<-connClosed
}

func TestServerErrorInvalidMethod(t *testing.T) {
	h := &testServerHandler{
		onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
			require.Equal(t, "unhandled request (INVALID rtsp://localhost:8554/)", ctx.Error.Error())
		},
	}

	s := &Server{Handler: h}
	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	res, err := writeReqReadRes(bconn, base.Request{
		Method: "INVALID",
		URL:    base.MustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)
}

func TestServerErrorTCPTwoConnOneSession(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
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
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn1, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn1.Close()
	bconn1 := bufio.NewReadWriter(bufio.NewReader(conn1), bufio.NewWriter(conn1))

	res, err := writeReqReadRes(bconn1, base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
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

	res, err = writeReqReadRes(bconn1, base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	conn2, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn2.Close()
	bconn2 := bufio.NewReadWriter(bufio.NewReader(conn2), bufio.NewWriter(conn2))

	res, err = writeReqReadRes(bconn2, base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)
}

func TestServerErrorTCPOneConnTwoSessions(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
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
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
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

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": res.Header["Session"],
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
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
				params = ctx.Req.Body
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
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Options,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.SetParameter,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"12"},
		},
		Body: []byte("param1: 123456\r\n"),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	res, err = writeReqReadRes(bconn, base.Request{
		Method: base.GetParameter,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
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
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			res, err := writeReqReadRes(bconn, base.Request{
				Method: method,
				URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq": base.HeaderValue{"1"},
				},
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusBadRequest, res.StatusCode)
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
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	err = base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
					return &v
				}(),
				Mode: func() *headers.TransportMode {
					v := headers.TransportModePlay
					return &v
				}(),
				InterleavedIDs: &[2]int{0, 1},
			}.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	<-sessionClosed
}

func TestServerSessionAutoClose(t *testing.T) {
	sessionClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onSessionClose: func(ctx *ServerHandlerOnSessionCloseCtx) {
				close(sessionClosed)
			},
			onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
		},
	}

	err := s.Start("127.0.0.1:8554")
	require.NoError(t, err)
	defer s.Close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	res, err := writeReqReadRes(bconn, base.Request{
		Method: base.Setup,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol: StreamProtocolTCP,
				Delivery: func() *base.StreamDelivery {
					v := base.StreamDeliveryUnicast
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

			s := &Server{
				Handler: &testServerHandler{
					onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
						require.Equal(t, "invalid path", ctx.Error.Error())
						close(connClosed)
					},
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onPlay: func(ctx *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
			}

			err := s.Start("127.0.0.1:8554")
			require.NoError(t, err)
			defer s.Close()

			conn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer conn.Close()
			bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

			sxID := ""

			if method == base.Record {
				track, err := NewTrackH264(96, []byte("123456"), []byte("123456"))
				require.NoError(t, err)

				tracks := Tracks{track}
				for i, t := range tracks {
					t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
						Key:   "control",
						Value: "trackID=" + strconv.FormatInt(int64(i), 10),
					})
				}

				res, err := writeReqReadRes(bconn, base.Request{
					Method: base.Announce,
					URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
					Header: base.Header{
						"CSeq":         base.HeaderValue{"1"},
						"Content-Type": base.HeaderValue{"application/sdp"},
					},
					Body: tracks.Write(),
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)
				sxID = res.Header["Session"][0]
			}

			if method == base.Play || method == base.Record || method == base.Pause {
				res, err := writeReqReadRes(bconn, base.Request{
					Method: base.Setup,
					URL:    base.MustParseURL("rtsp://localhost:8554/teststream/trackID=0"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"2"},
						"Session": base.HeaderValue{sxID},
						"Transport": headers.Transport{
							Protocol: StreamProtocolTCP,
							Delivery: func() *base.StreamDelivery {
								v := base.StreamDeliveryUnicast
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
				sxID = res.Header["Session"][0]
			}

			if method == base.Pause {
				res, err := writeReqReadRes(bconn, base.Request{
					Method: base.Play,
					URL:    base.MustParseURL("rtsp://localhost:8554/teststream/"),
					Header: base.Header{
						"CSeq":    base.HeaderValue{"2"},
						"Session": base.HeaderValue{sxID},
					},
				})
				require.NoError(t, err)
				require.Equal(t, base.StatusOK, res.StatusCode)
			}

			res, err := writeReqReadRes(bconn, base.Request{
				Method: method,
				URL:    base.MustParseURL("rtsp://localhost:8554"),
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
