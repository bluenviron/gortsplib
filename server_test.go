package gortsplib

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/auth"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/conn"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

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

func writeReqReadRes(
	conn *conn.Conn,
	req base.Request,
) (*base.Response, error) {
	err := conn.WriteRequest(&req)
	if err != nil {
		return nil, err
	}

	return conn.ReadResponse()
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
	onSetParameter func(*ServerHandlerOnSetParameterCtx) (*base.Response, error)
	onGetParameter func(*ServerHandlerOnGetParameterCtx) (*base.Response, error)
	onPacketLost   func(*ServerHandlerOnPacketLostCtx)
	onDecodeError  func(*ServerHandlerOnDecodeErrorCtx)
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

func (sh *testServerHandler) OnPacketLost(ctx *ServerHandlerOnPacketLostCtx) {
	if sh.onPacketLost != nil {
		sh.onPacketLost(ctx)
	}
}

func (sh *testServerHandler) OnDecodeError(ctx *ServerHandlerOnDecodeErrorCtx) {
	if sh.onDecodeError != nil {
		sh.onDecodeError(ctx)
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
	nconnClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnOpen: func(ctx *ServerHandlerOnConnOpenCtx) {
				ctx.Conn.Close()
				ctx.Conn.Close()
			},
			onConnClose: func(_ *ServerHandlerOnConnCloseCtx) {
				close(nconnClosed)
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	<-nconnClosed

	_, err = writeReqReadRes(conn, base.Request{
		Method: base.Options,
		URL:    mustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.Error(t, err)
}

func TestServerCSeq(t *testing.T) {
	s := &Server{
		RTSPAddress: "localhost:8554",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	res, err := writeReqReadRes(conn, base.Request{
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
	nconnClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				require.EqualError(t, ctx.Error, "CSeq is missing")
				close(nconnClosed)
			},
		},
		RTSPAddress: "localhost:8554",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Options,
		URL:    mustParseURL("rtsp://localhost:8554/"),
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	<-nconnClosed
}

func TestServerErrorNilURL(t *testing.T) {
	nconnClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				require.EqualError(t, ctx.Error, "invalid path")
				close(nconnClosed)
			},
		},
		RTSPAddress: "localhost:8554",
	}
	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Describe,
		URL:    nil,
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)

	<-nconnClosed
}

type testServerErrMethodNotImplemented struct {
	stream *ServerStream
}

func (s *testServerErrMethodNotImplemented) OnDescribe(
	_ *ServerHandlerOnDescribeCtx,
) (*base.Response, *ServerStream, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, s.stream, nil
}

func (s *testServerErrMethodNotImplemented) OnSetup(
	_ *ServerHandlerOnSetupCtx,
) (*base.Response, *ServerStream, error) {
	return &base.Response{
		StatusCode: base.StatusOK,
	}, s.stream, nil
}

func TestServerErrorMethodNotImplemented(t *testing.T) {
	for _, ca := range []string{"outside session", "inside session"} {
		t.Run(ca, func(t *testing.T) {
			h := &testServerErrMethodNotImplemented{}

			s := &Server{
				Handler:     h,
				RTSPAddress: "localhost:8554",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			stream := NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
			defer stream.Close()

			h.stream = stream

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			var session string

			if ca == "inside session" {
				inTH := &headers.Transport{
					Protocol:       headers.TransportProtocolTCP,
					Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
					Mode:           transportModePtr(headers.TransportModePlay),
					InterleavedIDs: &[2]int{0, 1},
				}

				res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

				session = readSession(t, res)
			}

			headers := base.Header{
				"CSeq": base.HeaderValue{"2"},
			}
			if ca == "inside session" {
				headers["Session"] = base.HeaderValue{session}
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.SetParameter,
				URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
				Header: headers,
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusNotImplemented, res.StatusCode)

			headers = base.Header{
				"CSeq": base.HeaderValue{"3"},
			}
			if ca == "inside session" {
				headers["Session"] = base.HeaderValue{session}
			}

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.Options,
				URL:    mustParseURL("rtsp://localhost:8554/teststream/"),
				Header: headers,
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerErrorTCPTwoConnOneSession(t *testing.T) {
	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(_ *ServerHandlerOnPauseCtx) (*base.Response, error) {
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

	stream = NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
	defer stream.Close()

	nconn1, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn1.Close()
	conn1 := conn.NewConn(nconn1)

	desc1 := doDescribe(t, conn1)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:           transportModePtr(headers.TransportModePlay),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, _ := doSetup(t, conn1, mediaURL(t, desc1.BaseURL, desc1.Medias[0]).String(), inTH, "")

	session := readSession(t, res)

	doPlay(t, conn1, "rtsp://localhost:8554/teststream", session)

	nconn2, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn2.Close()
	conn2 := conn.NewConn(nconn2)

	desc2 := doDescribe(t, conn2)

	res, err = writeReqReadRes(conn2, base.Request{
		Method: base.Setup,
		URL:    mediaURL(t, desc2.BaseURL, desc2.Medias[0]),
		Header: base.Header{
			"CSeq": base.HeaderValue{"1"},
			"Transport": headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
				Mode:           transportModePtr(headers.TransportModePlay),
				InterleavedIDs: &[2]int{0, 1},
			}.Marshal(),
			"Session": base.HeaderValue{session},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)
}

func TestServerErrorTCPOneConnTwoSessions(t *testing.T) {
	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, nil
			},
			onPause: func(_ *ServerHandlerOnPauseCtx) (*base.Response, error) {
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

	stream = NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:           transportModePtr(headers.TransportModePlay),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	session := readSession(t, res)

	doPlay(t, conn, "rtsp://localhost:8554/teststream", session)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
			"Transport": headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
				Mode:           transportModePtr(headers.TransportModePlay),
				InterleavedIDs: &[2]int{0, 1},
			}.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusBadRequest, res.StatusCode)
}

func TestServerSetupMultipleTransports(t *testing.T) {
	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	stream = NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTHS := headers.Transports{
		{
			Delivery:    deliveryPtr(headers.TransportDeliveryUnicast),
			Mode:        transportModePtr(headers.TransportModePlay),
			Protocol:    headers.TransportProtocolUDP,
			ClientPorts: &[2]int{35466, 35467},
		},
		{
			Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
			Mode:           transportModePtr(headers.TransportModePlay),
			Protocol:       headers.TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		},
	}

	res, err := writeReqReadRes(conn, base.Request{
		Method: base.Setup,
		URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
		Header: base.Header{
			"CSeq":      base.HeaderValue{"1"},
			"Transport": inTHS.Marshal(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)

	var th headers.Transport
	err = th.Unmarshal(res.Header["Transport"])
	require.NoError(t, err)
	require.Equal(t, headers.Transport{
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Protocol:       headers.TransportProtocolTCP,
		InterleavedIDs: &[2]int{0, 1},
	}, th)
}

func TestServerGetSetParameter(t *testing.T) {
	for _, ca := range []string{"inside session", "outside session"} {
		t.Run(ca, func(t *testing.T) {
			var stream *ServerStream
			var params []byte

			s := &Server{
				Handler: &testServerHandler{
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(ctx *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						ctx.Session.SetUserData(123)
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetParameter: func(ctx *ServerHandlerOnSetParameterCtx) (*base.Response, error) {
						if ca == "inside session" {
							require.NotNil(t, ctx.Session)
							require.Equal(t, 123, ctx.Session.UserData())
						} else {
							ctx.Conn.SetUserData(456)
						}
						params = ctx.Request.Body
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onGetParameter: func(ctx *ServerHandlerOnGetParameterCtx) (*base.Response, error) {
						if ca == "inside session" {
							require.NotNil(t, ctx.Session)
						} else {
							require.Equal(t, 456, ctx.Conn.UserData())
						}
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

			stream = NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			var session string

			if ca == "inside session" {
				inTH := &headers.Transport{
					Protocol:       headers.TransportProtocolTCP,
					Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
					Mode:           transportModePtr(headers.TransportModePlay),
					InterleavedIDs: &[2]int{0, 1},
				}

				res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

				session = readSession(t, res)
			}

			headers := base.Header{
				"CSeq": base.HeaderValue{"3"},
			}
			if ca == "inside session" {
				headers["Session"] = base.HeaderValue{session}
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.SetParameter,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: headers,
				Body:   []byte("param1: 123456\r\n"),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)

			headers = base.Header{
				"CSeq": base.HeaderValue{"4"},
			}
			if ca == "inside session" {
				headers["Session"] = base.HeaderValue{session}
			}

			res, err = writeReqReadRes(conn, base.Request{
				Method: base.GetParameter,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: headers,
				Body:   []byte("param1\r\n"),
			})
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
			require.Equal(t, []byte("param1: 123456\r\n"), res.Body)
		})
	}
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
					onPlay: func(_ *ServerHandlerOnPlayCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onRecord: func(_ *ServerHandlerOnRecordCtx) (*base.Response, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
					onPause: func(_ *ServerHandlerOnPauseCtx) (*base.Response, error) {
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

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			res, err := writeReqReadRes(conn, base.Request{
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
	var stream *ServerStream
	var session *ServerSession
	connClosed := make(chan struct{})

	s := &Server{
		Handler: &testServerHandler{
			onSessionOpen: func(ctx *ServerHandlerOnSessionOpenCtx) {
				session = ctx.Session
			},
			onConnClose: func(_ *ServerHandlerOnConnCloseCtx) {
				close(connClosed)
			},
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	stream = NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:           transportModePtr(headers.TransportModePlay),
		InterleavedIDs: &[2]int{0, 1},
	}

	doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	session.Close()
	session.Close()

	select {
	case <-connClosed:
	case <-time.After(2 * time.Second):
		t.Errorf("should not happen")
	}

	_, err = writeReqReadRes(conn, base.Request{
		Method: base.Options,
		URL:    mustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"2"},
		},
	})
	require.Error(t, err)
}

func TestServerSessionAutoClose(t *testing.T) {
	for _, ca := range []string{
		"200", "400",
	} {
		t.Run(ca, func(t *testing.T) {
			var stream *ServerStream
			sessionClosed := make(chan struct{})

			s := &Server{
				Handler: &testServerHandler{
					onSessionClose: func(_ *ServerHandlerOnSessionCloseCtx) {
						close(sessionClosed)
					},
					onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
						return &base.Response{
							StatusCode: base.StatusOK,
						}, stream, nil
					},
					onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
						if ca == "200" {
							return &base.Response{
								StatusCode: base.StatusOK,
							}, stream, nil
						}

						return &base.Response{
							StatusCode: base.StatusBadRequest,
						}, nil, fmt.Errorf("error")
					},
				},
				RTSPAddress: "localhost:8554",
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			stream = NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
			defer stream.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			conn := conn.NewConn(nconn)

			desc := doDescribe(t, conn)

			inTH := &headers.Transport{
				Protocol:       headers.TransportProtocolTCP,
				Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
				Mode:           transportModePtr(headers.TransportModePlay),
				InterleavedIDs: &[2]int{0, 1},
			}

			res, err := writeReqReadRes(conn, base.Request{
				Method: base.Setup,
				URL:    mediaURL(t, desc.BaseURL, desc.Medias[0]),
				Header: base.Header{
					"CSeq":      base.HeaderValue{"1"},
					"Transport": inTH.Marshal(),
				},
			})
			require.NoError(t, err)

			if ca == "200" {
				require.Equal(t, base.StatusOK, res.StatusCode)
			} else {
				require.Equal(t, base.StatusBadRequest, res.StatusCode)
			}

			nconn.Close()

			<-sessionClosed
		})
	}
}

func TestServerSessionTeardown(t *testing.T) {
	var stream *ServerStream

	s := &Server{
		Handler: &testServerHandler{
			onDescribe: func(_ *ServerHandlerOnDescribeCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
			onSetup: func(_ *ServerHandlerOnSetupCtx) (*base.Response, *ServerStream, error) {
				return &base.Response{
					StatusCode: base.StatusOK,
				}, stream, nil
			},
		},
		RTSPAddress: "localhost:8554",
	}

	err := s.Start()
	require.NoError(t, err)
	defer s.Close()

	stream = NewServerStream(s, &description.Session{Medias: []*description.Media{testH264Media}})
	defer stream.Close()

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	desc := doDescribe(t, conn)

	inTH := &headers.Transport{
		Protocol:       headers.TransportProtocolTCP,
		Delivery:       deliveryPtr(headers.TransportDeliveryUnicast),
		Mode:           transportModePtr(headers.TransportModePlay),
		InterleavedIDs: &[2]int{0, 1},
	}

	res, _ := doSetup(t, conn, mediaURL(t, desc.BaseURL, desc.Medias[0]).String(), inTH, "")

	session := readSession(t, res)

	doTeardown(t, conn, "rtsp://localhost:8554/", session)

	res, err = writeReqReadRes(conn, base.Request{
		Method: base.Options,
		URL:    mustParseURL("rtsp://localhost:8554/"),
		Header: base.Header{
			"CSeq": base.HeaderValue{"3"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, base.StatusOK, res.StatusCode)
}

func TestServerAuth(t *testing.T) {
	for _, method := range []string{"all", "basic", "digest_md5", "digest_sha256"} {
		t.Run(method, func(t *testing.T) {
			s := &Server{
				Handler: &testServerHandler{
					onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
						ok := ctx.Conn.VerifyCredentials(ctx.Request, "myuser", "mypass")
						if !ok {
							return &base.Response{
								StatusCode: base.StatusUnauthorized,
							}, liberrors.ErrServerAuth{}
						}

						return &base.Response{
							StatusCode: base.StatusOK,
						}, nil
					},
				},
				RTSPAddress: "localhost:8554",
				AuthMethods: func() []auth.VerifyMethod {
					switch method {
					case "basic":
						return []auth.VerifyMethod{auth.VerifyMethodBasic}

					case "digest_md5":
						return []auth.VerifyMethod{auth.VerifyMethodDigestMD5}

					case "digest_sha256":
						return []auth.VerifyMethod{auth.VerifyMethodDigestSHA256}
					}
					return nil
				}(),
			}

			err := s.Start()
			require.NoError(t, err)
			defer s.Close()

			nconn, err := net.Dial("tcp", "localhost:8554")
			require.NoError(t, err)
			defer nconn.Close()
			conn := conn.NewConn(nconn)

			medias := []*description.Media{testH264Media}

			req := base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://localhost:8554/teststream"),
				Header: base.Header{
					"CSeq":         base.HeaderValue{"1"},
					"Content-Type": base.HeaderValue{"application/sdp"},
				},
				Body: mediasToSDP(medias),
			}

			res, err := writeReqReadRes(conn, req)
			require.NoError(t, err)
			require.Equal(t, base.StatusUnauthorized, res.StatusCode)

			sender, err := auth.NewSender(res.Header["WWW-Authenticate"], "myuser", "mypass")
			require.NoError(t, err)

			sender.AddAuthorization(&req)
			res, err = writeReqReadRes(conn, req)
			require.NoError(t, err)
			require.Equal(t, base.StatusOK, res.StatusCode)
		})
	}
}

func TestServerAuthFail(t *testing.T) {
	s := &Server{
		Handler: &testServerHandler{
			onConnClose: func(ctx *ServerHandlerOnConnCloseCtx) {
				require.EqualError(t, ctx.Error, "authentication error")
			},
			onAnnounce: func(ctx *ServerHandlerOnAnnounceCtx) (*base.Response, error) {
				ok := ctx.Conn.VerifyCredentials(ctx.Request, "myuser2", "mypass2")
				if !ok {
					return &base.Response{
						StatusCode: http.StatusUnauthorized,
					}, liberrors.ErrServerAuth{}
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

	nconn, err := net.Dial("tcp", "localhost:8554")
	require.NoError(t, err)
	defer nconn.Close()
	conn := conn.NewConn(nconn)

	medias := []*description.Media{testH264Media}

	req := base.Request{
		Method: base.Announce,
		URL:    mustParseURL("rtsp://localhost:8554/teststream"),
		Header: base.Header{
			"CSeq":         base.HeaderValue{"1"},
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: mediasToSDP(medias),
	}

	res, err := writeReqReadRes(conn, req)
	require.NoError(t, err)
	require.Equal(t, base.StatusUnauthorized, res.StatusCode)

	sender, err := auth.NewSender(res.Header["WWW-Authenticate"], "myuser", "mypass")
	require.NoError(t, err)

	sender.AddAuthorization(&req)

	res, err = writeReqReadRes(conn, req)
	require.NoError(t, err)
	require.Equal(t, base.StatusUnauthorized, res.StatusCode)

	_, err = writeReqReadRes(conn, req)
	require.Error(t, err)
}
