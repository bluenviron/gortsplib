package gortsplib

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

type testServ struct {
	s               *Server
	udpRTPListener  *ServerUDPListener
	udpRTCPListener *ServerUDPListener
	wg              sync.WaitGroup
	mutex           sync.Mutex
	publisher       *ServerConn
	sdp             []byte
	readers         map[*ServerConn]struct{}
}

func newTestServ(tlsConf *tls.Config) (*testServ, error) {
	var conf ServerConf
	var udpRTPListener *ServerUDPListener
	var udpRTCPListener *ServerUDPListener
	if tlsConf != nil {
		conf = ServerConf{
			TLSConfig: tlsConf,
		}

	} else {
		var err error
		udpRTPListener, err = NewServerUDPListener(":8000")
		if err != nil {
			return nil, err
		}

		udpRTCPListener, err = NewServerUDPListener(":8001")
		if err != nil {
			return nil, err
		}

		conf = ServerConf{
			UDPRTPListener:  udpRTPListener,
			UDPRTCPListener: udpRTCPListener,
		}
	}

	s, err := conf.Serve(":8554")
	if err != nil {
		return nil, err
	}

	ts := &testServ{
		s:               s,
		udpRTPListener:  udpRTPListener,
		udpRTCPListener: udpRTCPListener,
		readers:         make(map[*ServerConn]struct{}),
	}

	ts.wg.Add(1)
	go ts.run()

	return ts, nil
}

func (ts *testServ) close() {
	ts.s.Close()
	ts.wg.Wait()
	if ts.udpRTPListener != nil {
		ts.udpRTPListener.Close()
	}
	if ts.udpRTCPListener != nil {
		ts.udpRTCPListener.Close()
	}
}

func (ts *testServ) run() {
	defer ts.wg.Done()

	for {
		conn, err := ts.s.Accept()
		if err != nil {
			return
		}

		ts.wg.Add(1)
		go ts.handleConn(conn)
	}
}

func (ts *testServ) handleConn(conn *ServerConn) {
	defer ts.wg.Done()
	defer conn.Close()

	onDescribe := func(req *base.Request) (*base.Response, error) {
		ts.mutex.Lock()
		defer ts.mutex.Unlock()

		if ts.publisher == nil {
			return &base.Response{
				StatusCode: base.StatusNotFound,
			}, nil
		}

		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Base": base.HeaderValue{req.URL.String() + "/"},
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Content: ts.sdp,
		}, nil
	}

	onAnnounce := func(req *base.Request, tracks Tracks) (*base.Response, error) {
		ts.mutex.Lock()
		defer ts.mutex.Unlock()

		if ts.publisher != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, fmt.Errorf("someone is already publishing")
		}

		ts.publisher = conn
		ts.sdp = tracks.Write()

		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Session": base.HeaderValue{"12345678"},
			},
		}, nil
	}

	onSetup := func(req *base.Request, th *headers.Transport) (*base.Response, error) {
		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Session": base.HeaderValue{"12345678"},
			},
		}, nil
	}

	onPlay := func(req *base.Request) (*base.Response, error) {
		ts.mutex.Lock()
		defer ts.mutex.Unlock()

		ts.readers[conn] = struct{}{}

		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Session": base.HeaderValue{"12345678"},
			},
		}, nil
	}

	onRecord := func(req *base.Request) (*base.Response, error) {
		ts.mutex.Lock()
		defer ts.mutex.Unlock()

		if conn != ts.publisher {
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
	}

	onFrame := func(trackID int, typ StreamType, buf []byte) {
		ts.mutex.Lock()
		defer ts.mutex.Unlock()

		if conn == ts.publisher {
			for r := range ts.readers {
				r.WriteFrame(trackID, typ, buf)
			}
		}
	}

	<-conn.Read(ServerConnReadHandlers{
		OnDescribe: onDescribe,
		OnAnnounce: onAnnounce,
		OnSetup:    onSetup,
		OnPlay:     onPlay,
		OnRecord:   onRecord,
		OnFrame:    onFrame,
	})

	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	if conn == ts.publisher {
		ts.publisher = nil
		ts.sdp = nil
	}
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

func TestServerPublishRead(t *testing.T) {
	for _, ca := range []struct {
		encrypted      bool
		publisherSoft  string
		publisherProto string
		readerSoft     string
		readerProto    string
	}{
		{false, "ffmpeg", "udp", "ffmpeg", "udp"},
		{false, "ffmpeg", "udp", "gstreamer", "udp"},
		{false, "gstreamer", "udp", "ffmpeg", "udp"},
		{false, "gstreamer", "udp", "gstreamer", "udp"},

		{false, "ffmpeg", "tcp", "ffmpeg", "tcp"},
		{false, "ffmpeg", "tcp", "gstreamer", "tcp"},
		{false, "gstreamer", "tcp", "ffmpeg", "tcp"},
		{false, "gstreamer", "tcp", "gstreamer", "tcp"},

		{false, "ffmpeg", "tcp", "ffmpeg", "udp"},
		{false, "ffmpeg", "udp", "ffmpeg", "tcp"},

		{true, "ffmpeg", "tcp", "ffmpeg", "tcp"},
		{true, "ffmpeg", "tcp", "gstreamer", "tcp"},
		{true, "gstreamer", "tcp", "ffmpeg", "tcp"},
		{true, "gstreamer", "tcp", "gstreamer", "tcp"},
	} {
		encryptedStr := func() string {
			if ca.encrypted {
				return "encrypted"
			}
			return "plain"
		}()

		t.Run(encryptedStr+"_"+ca.publisherSoft+"_"+ca.publisherProto+"_"+
			ca.readerSoft+"_"+ca.readerProto, func(t *testing.T) {
			var proto string
			var tlsConf *tls.Config
			if !ca.encrypted {
				proto = "rtsp"
				tlsConf = nil

			} else {
				proto = "rtsps"
				cert, err := tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)
				tlsConf = &tls.Config{Certificates: []tls.Certificate{cert}}
			}

			ts, err := newTestServ(tlsConf)
			require.NoError(t, err)
			defer ts.close()

			switch ca.publisherSoft {
			case "ffmpeg":
				cnt1, err := newContainer("ffmpeg", "publish", []string{
					"-re",
					"-stream_loop", "-1",
					"-i", "emptyvideo.ts",
					"-c", "copy",
					"-f", "rtsp",
					"-rtsp_transport", ca.publisherProto,
					proto + "://localhost:8554/teststream",
				})
				require.NoError(t, err)
				defer cnt1.close()

			case "gstreamer":
				cnt1, err := newContainer("gstreamer", "publish", []string{
					"filesrc location=emptyvideo.ts ! tsdemux ! video/x-h264 ! rtspclientsink " +
						"location=" + proto + "://127.0.0.1:8554/teststream protocols=" + ca.publisherProto + " tls-validation-flags=0 latency=0 timeout=0 rtx-time=0",
				})
				require.NoError(t, err)
				defer cnt1.close()

				time.Sleep(1 * time.Second)
			}

			time.Sleep(1 * time.Second)

			switch ca.readerSoft {
			case "ffmpeg":
				cnt2, err := newContainer("ffmpeg", "read", []string{
					"-rtsp_transport", ca.readerProto,
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
					"rtspsrc location=" + proto + "://127.0.0.1:8554/teststream protocols=" + ca.readerProto + " tls-validation-flags=0 latency=0 " +
						"! application/x-rtp,media=video ! decodebin ! exitafterframe ! fakesink",
				})
				require.NoError(t, err)
				defer cnt2.close()
				require.Equal(t, 0, cnt2.wait())
			}
		})
	}
}

func TestServerTeardownResponse(t *testing.T) {
	ts, err := newTestServ(nil)
	require.NoError(t, err)
	defer ts.close()

	conn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer conn.Close()
	bconn := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	err = base.Request{
		Method: base.Teardown,
		URL:    base.MustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	buf := make([]byte, 2048)
	_, err = bconn.Read(buf)
	require.Equal(t, io.EOF, err)
}

func TestServerResponseBeforeFrames(t *testing.T) {
	ts, err := newTestServ(nil)
	require.NoError(t, err)
	defer ts.close()

	cnt1, err := newContainer("ffmpeg", "publish", []string{
		"-re",
		"-stream_loop", "-1",
		"-i", "emptyvideo.ts",
		"-c", "copy",
		"-f", "rtsp",
		"-rtsp_transport", "tcp",
		"rtsp://localhost:8554/teststream",
	})
	require.NoError(t, err)
	defer cnt1.close()

	time.Sleep(1 * time.Second)

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
				InterleavedIds: &[2]int{0, 1},
			}.Write(),
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	var res base.Response
	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	err = base.Request{
		Method: base.Play,
		URL:    base.MustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	}.Write(bconn.Writer)
	require.NoError(t, err)

	err = res.Read(bconn.Reader)
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var fr base.InterleavedFrame
	fr.Content = make([]byte, 2048)
	err = fr.Read(bconn.Reader)
	require.NoError(t, err)
}
