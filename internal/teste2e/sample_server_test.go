//go:build enable_e2e_tests

package teste2e

import (
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
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

type sampleServer struct {
	tlsConfig *tls.Config

	s         *gortsplib.Server
	mutex     sync.Mutex
	stream    *gortsplib.ServerStream
	publisher *gortsplib.ServerSession
}

func (sh *sampleServer) OnSessionClose(ctx *gortsplib.ServerHandlerOnSessionCloseCtx) {
	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sh.stream != nil {
		if ctx.Session == sh.publisher {
			sh.stream.Close()
			sh.stream = nil
		}
	}
}

func (sh *sampleServer) OnDescribe(ctx *gortsplib.ServerHandlerOnDescribeCtx) (*base.Response, *gortsplib.ServerStream, error) {
	if ctx.Path != "/test/stream" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, nil, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
	}
	if ctx.Query != "key=val" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, nil, fmt.Errorf("invalid query (%s)", ctx.Query)
	}

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

func (sh *sampleServer) OnAnnounce(ctx *gortsplib.ServerHandlerOnAnnounceCtx) (*base.Response, error) {
	if ctx.Path != "/test/stream" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
	}
	if ctx.Query != "key=val" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("invalid query (%s)", ctx.Query)
	}

	sh.mutex.Lock()
	defer sh.mutex.Unlock()

	if sh.stream != nil {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("someone is already publishing")
	}

	sh.stream = &gortsplib.ServerStream{
		Server: sh.s,
		Desc:   ctx.Description,
	}
	err := sh.stream.Initialize()
	if err != nil {
		return &base.Response{
			StatusCode: base.StatusInternalServerError,
		}, err
	}

	sh.publisher = ctx.Session

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (sh *sampleServer) OnSetup(ctx *gortsplib.ServerHandlerOnSetupCtx) (*base.Response, *gortsplib.ServerStream, error) {
	if ctx.Path != "/test/stream" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, nil, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
	}
	if ctx.Query != "key=val" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, nil, fmt.Errorf("invalid query (%s)", ctx.Query)
	}

	if ctx.Session.State() == gortsplib.ServerSessionStatePreRecord {
		return &base.Response{
			StatusCode: base.StatusOK,
		}, nil, nil
	}

	if sh.stream == nil {
		return &base.Response{
			StatusCode: base.StatusNotFound,
		}, nil, nil
	}

	return &base.Response{
		StatusCode: base.StatusOK,
	}, sh.stream, nil
}

func (sh *sampleServer) OnPlay(ctx *gortsplib.ServerHandlerOnPlayCtx) (*base.Response, error) {
	if ctx.Path != "/test/stream" {
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
}

func (sh *sampleServer) OnRecord(ctx *gortsplib.ServerHandlerOnRecordCtx) (*base.Response, error) {
	if ctx.Path != "/test/stream" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("invalid path (%s)", ctx.Request.URL)
	}
	if ctx.Query != "key=val" {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("invalid query (%s)", ctx.Query)
	}

	ctx.Session.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		sh.stream.WritePacketRTP(medi, pkt)
	})

	return &base.Response{
		StatusCode: base.StatusOK,
	}, nil
}

func (sh *sampleServer) initialize() error {
	sh.s = &gortsplib.Server{
		Handler:     sh,
		TLSConfig:   sh.tlsConfig,
		RTSPAddress: "0.0.0.0:8554",
	}

	if sh.tlsConfig == nil {
		sh.s.UDPRTPAddress = "0.0.0.0:8000"
		sh.s.UDPRTCPAddress = "0.0.0.0:8001"
		sh.s.MulticastIPRange = "224.1.0.0/16"
		sh.s.MulticastRTPPort = 8002
		sh.s.MulticastRTCPPort = 8003
	}

	err := sh.s.Start()
	if err != nil {
		return err
	}

	return nil
}

func (sh *sampleServer) close() {
	defer sh.s.Close()
}
