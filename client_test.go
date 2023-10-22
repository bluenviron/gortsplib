package gortsplib

import (
	"crypto/tls"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/auth"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/conn"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/url"
)

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestClientURLToAddress(t *testing.T) {
	for _, ca := range []struct {
		name string
		url  string
		addr string
	}{
		{
			"rtsp ipv6 with port",
			"rtsp://[::1]:8888/path",
			"[::1]:8888",
		},
		{
			"rtsp ipv6 without port",
			"rtsp://[::1]/path",
			"[::1]:554",
		},
		{
			"rtsps without port",
			"rtsps://2.2.2.2/path",
			"2.2.2.2:322",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			addr := canonicalAddr(mustParseURL(ca.url))
			require.Equal(t, ca.addr, addr)
		})
	}
}

func TestClientTLSSetServerName(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()

		cert, err := tls.X509KeyPair(serverCert, serverKey)
		require.NoError(t, err)

		tnconn := tls.Server(nconn, &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
			VerifyConnection: func(cs tls.ConnectionState) error {
				require.Equal(t, "localhost", cs.ServerName)
				return nil
			},
		})

		err = tnconn.Handshake()
		require.EqualError(t, err, "remote error: tls: bad certificate")
	}()

	u, err := url.Parse("rtsps://localhost:8554/stream")
	require.NoError(t, err)

	c := Client{
		TLSConfig: &tls.Config{},
	}

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	_, err = c.Options(u)
	require.Error(t, err)

	<-serverDone
}

func TestClientClose(t *testing.T) {
	u, err := url.Parse("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{}

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)

	c.Close()

	_, err = c.Options(u)
	require.EqualError(t, err, "terminated")

	_, _, err = c.Describe(u)
	require.EqualError(t, err, "terminated")

	_, err = c.Announce(u, nil)
	require.EqualError(t, err, "terminated")

	_, err = c.Setup(nil, nil, 0, 0)
	require.EqualError(t, err, "terminated")

	_, err = c.Play(nil)
	require.EqualError(t, err, "terminated")

	_, err = c.Record()
	require.EqualError(t, err, "terminated")

	_, err = c.Pause()
	require.EqualError(t, err, "terminated")
}

func TestClientCloseDuringRequest(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	requestReceived := make(chan struct{})
	releaseConn := make(chan struct{})

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		close(requestReceived)
		<-releaseConn
	}()

	u, err := url.Parse("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{}

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)

	optionsDone := make(chan struct{})
	go func() {
		defer close(optionsDone)
		_, err := c.Options(u)
		require.Error(t, err)
	}()

	<-requestReceived
	c.Close()
	<-optionsDone
	close(releaseConn)
}

func TestClientSession(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		conn := conn.NewConn(nconn)
		defer nconn.Close()

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
				"Session": base.HeaderValue{"123456"},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, base.HeaderValue{"123456"}, req.Header["Session"])

		medias := []*description.Media{testH264Media}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Session":      base.HeaderValue{"123456"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err)
	}()

	u, err := url.Parse("rtsp://localhost:8554/stream")
	require.NoError(t, err)

	c := Client{}

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	_, _, err = c.Describe(u)
	require.NoError(t, err)
}

func TestClientAuth(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		conn := conn.NewConn(nconn)
		defer nconn.Close()

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		nonce, err := auth.GenerateNonce()
		require.NoError(t, err)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusUnauthorized,
			Header: base.Header{
				"WWW-Authenticate": auth.GenerateWWWAuthenticate(nil, "IPCAM", nonce),
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)

		err = auth.Validate(req, "myuser", "mypass", nil, nil, "IPCAM", nonce)
		require.NoError(t, err)

		medias := []*description.Media{testH264Media}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err)
	}()

	u, err := url.Parse("rtsp://myuser:mypass@localhost:8554/stream")
	require.NoError(t, err)

	c := Client{}

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	_, _, err = c.Describe(u)
	require.NoError(t, err)
}

func TestClientCSeq(t *testing.T) {
	for _, ca := range []string{
		"different cseq",
		"space at the end",
	} {
		t.Run(ca, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()
			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				defer nconn.Close()
				conn := conn.NewConn(nconn)

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				switch ca {
				case "different cseq":
					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
							"CSeq": base.HeaderValue{"150"},
						},
					})
					require.NoError(t, err)

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
							"CSeq": req.Header["CSeq"],
						},
					})
					require.NoError(t, err)

				case "space at the end":
					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
							"CSeq": base.HeaderValue{req.Header["CSeq"][0] + " "},
						},
					})
					require.NoError(t, err)
				}
			}()

			u, err := url.Parse("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			c := Client{}

			err = c.Start(u.Scheme, u.Host)
			require.NoError(t, err)
			defer c.Close()

			_, err = c.Options(u)
			require.NoError(t, err)
		})
	}
}

func TestClientDescribeCharset(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()
	go func() {
		defer close(serverDone)

		nconn, err := l.Accept()
		require.NoError(t, err)
		defer nconn.Close()
		conn := conn.NewConn(nconn)

		req, err := conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Options, req.Method)

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
			},
		})
		require.NoError(t, err)

		req, err = conn.ReadRequest()
		require.NoError(t, err)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		medias := []*description.Media{testH264Media}

		err = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp; charset=utf-8"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err)
	}()

	u, err := url.Parse("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{}

	err = c.Start(u.Scheme, u.Host)
	require.NoError(t, err)
	defer c.Close()

	_, _, err = c.Describe(u)
	require.NoError(t, err)
}

func TestClientReplyToServerRequest(t *testing.T) {
	for _, ca := range []string{"after response", "before response"} {
		t.Run(ca, func(t *testing.T) {
			l, err := net.Listen("tcp", "localhost:8554")
			require.NoError(t, err)
			defer l.Close()

			serverDone := make(chan struct{})

			go func() {
				defer close(serverDone)

				nconn, err := l.Accept()
				require.NoError(t, err)
				conn := conn.NewConn(nconn)
				defer nconn.Close()

				req, err := conn.ReadRequest()
				require.NoError(t, err)
				require.Equal(t, base.Options, req.Method)

				if ca == "after response" {
					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
						},
					})
					require.NoError(t, err)

					err = conn.WriteRequest(&base.Request{
						Method: base.Options,
						URL:    nil,
						Header: base.Header{
							"CSeq": base.HeaderValue{"4"},
						},
					})
					require.NoError(t, err)

					res, err := conn.ReadResponse()
					require.NoError(t, err)
					require.Equal(t, base.StatusOK, res.StatusCode)
					require.Equal(t, "4", res.Header["CSeq"][0])
				} else {
					err = conn.WriteRequest(&base.Request{
						Method: base.Options,
						URL:    nil,
						Header: base.Header{
							"CSeq": base.HeaderValue{"4"},
						},
					})
					require.NoError(t, err)

					res, err := conn.ReadResponse()
					require.NoError(t, err)
					require.Equal(t, base.StatusOK, res.StatusCode)
					require.Equal(t, "4", res.Header["CSeq"][0])

					err = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
						},
					})
					require.NoError(t, err)
				}
			}()

			u, err := url.Parse("rtsp://localhost:8554/stream")
			require.NoError(t, err)

			c := Client{}

			err = c.Start(u.Scheme, u.Host)
			require.NoError(t, err)
			defer c.Close()

			_, err = c.Options(u)
			require.NoError(t, err)

			<-serverDone
		})
	}
}
