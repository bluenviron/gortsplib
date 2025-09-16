package gortsplib

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/internal/base64streamreader"
	"github.com/bluenviron/gortsplib/v5/pkg/auth"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/conn"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
)

func mustParseURL(s string) *base.URL {
	u, err := base.ParseURL(s)
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

func TestClientClose(t *testing.T) {
	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	err = c.Start()
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		close(requestReceived)
		<-releaseConn
	}()

	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	err = c.Start()
	require.NoError(t, err)

	optionsDone := make(chan struct{})
	go func() {
		defer close(optionsDone)
		_, err2 := c.Options(u)
		require.Error(t, err2)
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)
		defer nconn.Close()

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
				"Session": base.HeaderValue{"123456"},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, base.HeaderValue{"123456"}, req.Header["Session"])

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
				"Session":      base.HeaderValue{"123456"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)
	}()

	u, err := base.ParseURL("rtsp://localhost:8554/stream")
	require.NoError(t, err)

	c := Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	err = c.Start()
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)
		defer nconn.Close()

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)

		nonce, err2 := auth.GenerateNonce()
		require.NoError(t, err2)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusUnauthorized,
			Header: base.Header{
				"WWW-Authenticate": auth.GenerateWWWAuthenticate(nil, "IPCAM", nonce),
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)

		err2 = auth.Verify(req, "myuser", "mypass", nil, "IPCAM", nonce)
		require.NoError(t, err2)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)
	}()

	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/stream")
	require.NoError(t, err)

	c := Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	err = c.Start()
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

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn.Close()
				conn := conn.NewConn(bufio.NewReader(nconn), nconn)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				switch ca {
				case "different cseq":
					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
							"CSeq": base.HeaderValue{"150"},
						},
					})
					require.NoError(t, err2)

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
							"CSeq": req.Header["CSeq"],
						},
					})
					require.NoError(t, err2)

				case "space at the end":
					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
							"CSeq": base.HeaderValue{req.Header["CSeq"][0] + " "},
						},
					})
					require.NoError(t, err2)
				}
			}()

			u, err := base.ParseURL("rtsp://localhost:8554/teststream")
			require.NoError(t, err)

			c := Client{
				Scheme: u.Scheme,
				Host:   u.Host,
			}

			err = c.Start()
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

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp; charset=utf-8"},
				"Content-Base": base.HeaderValue{"rtsp://localhost:8554/teststream/"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)
	}()

	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	err = c.Start()
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

				nconn, err2 := l.Accept()
				require.NoError(t, err2)
				conn := conn.NewConn(bufio.NewReader(nconn), nconn)
				defer nconn.Close()

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				if ca == "after response" {
					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
						},
					})
					require.NoError(t, err2)

					err2 = conn.WriteRequest(&base.Request{
						Method: base.Options,
						URL:    nil,
						Header: base.Header{
							"CSeq": base.HeaderValue{"4"},
						},
					})
					require.NoError(t, err2)

					var res *base.Response
					res, err2 = conn.ReadResponse()
					require.NoError(t, err2)
					require.Equal(t, base.StatusOK, res.StatusCode)
					require.Equal(t, "4", res.Header["CSeq"][0])
				} else {
					err2 = conn.WriteRequest(&base.Request{
						Method: base.Options,
						URL:    nil,
						Header: base.Header{
							"CSeq": base.HeaderValue{"4"},
						},
					})
					require.NoError(t, err2)

					var res *base.Response
					res, err2 = conn.ReadResponse()
					require.NoError(t, err2)
					require.Equal(t, base.StatusOK, res.StatusCode)
					require.Equal(t, "4", res.Header["CSeq"][0])

					err2 = conn.WriteResponse(&base.Response{
						StatusCode: base.StatusOK,
						Header: base.Header{
							"Public": base.HeaderValue{strings.Join([]string{
								string(base.Describe),
							}, ", ")},
						},
					})
					require.NoError(t, err2)
				}
			}()

			u, err := base.ParseURL("rtsp://localhost:8554/stream")
			require.NoError(t, err)

			c := Client{
				Scheme: u.Scheme,
				Host:   u.Host,
			}

			err = c.Start()
			require.NoError(t, err)
			defer c.Close()

			_, err = c.Options(u)
			require.NoError(t, err)

			<-serverDone
		})
	}
}

func TestClientRelativeContentBase(t *testing.T) {
	l, err := net.Listen("tcp", "localhost:8554")
	require.NoError(t, err)
	defer l.Close()

	serverDone := make(chan struct{})
	defer func() { <-serverDone }()

	go func() {
		defer close(serverDone)

		nconn, err2 := l.Accept()
		require.NoError(t, err2)
		defer nconn.Close()
		conn := conn.NewConn(bufio.NewReader(nconn), nconn)

		req, err2 := conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Options, req.Method)

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join([]string{
					string(base.Describe),
				}, ", ")},
			},
		})
		require.NoError(t, err2)

		req, err2 = conn.ReadRequest()
		require.NoError(t, err2)
		require.Equal(t, base.Describe, req.Method)
		require.Equal(t, mustParseURL("rtsp://localhost:8554/teststream"), req.URL)

		medias := []*description.Media{testH264Media}

		err2 = conn.WriteResponse(&base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"application/sdp; charset=utf-8"},
				"Content-Base": base.HeaderValue{"/relative-content-base"},
			},
			Body: mediasToSDP(medias),
		})
		require.NoError(t, err2)
	}()

	u, err := base.ParseURL("rtsp://localhost:8554/teststream")
	require.NoError(t, err)

	c := Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	err = c.Start()
	require.NoError(t, err)
	defer c.Close()

	desc, _, err := c.Describe(u)
	require.NoError(t, err)

	require.Equal(t, "rtsp://localhost:8554/relative-content-base", desc.BaseURL.String())
}

func TestClientHTTPTunnel(t *testing.T) {
	for _, ca := range []string{"http", "https"} {
		t.Run(ca, func(t *testing.T) {
			var l net.Listener
			var err error

			if ca == "http" {
				l, err = net.Listen("tcp", "localhost:8554")
				require.NoError(t, err)
				defer l.Close()
			} else {
				var cert tls.Certificate
				cert, err = tls.X509KeyPair(serverCert, serverKey)
				require.NoError(t, err)

				l, err = tls.Listen("tcp", "localhost:8554", &tls.Config{Certificates: []tls.Certificate{cert}})
				require.NoError(t, err)
				defer l.Close()
			}

			var scheme string
			if ca == "http" {
				scheme = "rtsp"
			} else {
				scheme = "rtsps"
			}

			serverDone := make(chan struct{})
			defer func() { <-serverDone }()

			go func() {
				defer close(serverDone)

				nconn1, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn1.Close()

				buf1 := bufio.NewReader(nconn1)
				req1, err2 := http.ReadRequest(buf1)
				require.NoError(t, err2)

				require.Equal(t, &http.Request{
					Method:     http.MethodGet,
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					URL: &url.URL{
						Path: "/",
					},
					Host:       "localhost:8554",
					RequestURI: "/",
					Header: http.Header{
						"Accept":          []string{"application/x-rtsp-tunnelled"},
						"Content-Length":  []string{"30000"},
						"X-Sessioncookie": req1.Header["X-Sessioncookie"],
					},
					ContentLength: 30000,
					Body:          req1.Body,
				}, req1)

				require.NotEmpty(t, req1.Header.Get("X-Sessioncookie"))

				h := http.Header{}
				h.Set("Cache-Control", "no-cache")
				h.Set("Connection", "close")
				h.Set("Content-Type", "application/x-rtsp-tunnelled")
				h.Set("Pragma", "no-cache")
				res := http.Response{
					StatusCode:    http.StatusOK,
					ProtoMajor:    1,
					ProtoMinor:    req1.ProtoMinor,
					Header:        h,
					ContentLength: -1,
				}
				var resBuf bytes.Buffer
				res.Write(&resBuf) //nolint:errcheck
				_, err = nconn1.Write(resBuf.Bytes())
				require.NoError(t, err)

				nconn2, err2 := l.Accept()
				require.NoError(t, err2)
				defer nconn2.Close()

				buf2 := bufio.NewReader(nconn2)
				req2, err2 := http.ReadRequest(buf2)
				require.NoError(t, err2)

				require.Equal(t, &http.Request{
					Method:     http.MethodPost,
					Proto:      "HTTP/1.1",
					ProtoMajor: 1,
					ProtoMinor: 1,
					URL: &url.URL{
						Path: "/",
					},
					Host:       "localhost:8554",
					RequestURI: "/",
					Header: http.Header{
						"Content-Type":    []string{"application/x-rtsp-tunnelled"},
						"Content-Length":  []string{"30000"},
						"X-Sessioncookie": req2.Header["X-Sessioncookie"],
					},
					ContentLength: 30000,
					Body:          req2.Body,
				}, req2)

				require.Equal(t, req1.Header.Get("X-Sessioncookie"), req2.Header.Get("X-Sessioncookie"))

				h = http.Header{}
				h.Set("Cache-Control", "no-cache")
				h.Set("Connection", "close")
				h.Set("Content-Type", "application/x-rtsp-tunnelled")
				h.Set("Pragma", "no-cache")
				res = http.Response{
					StatusCode:    http.StatusOK,
					ProtoMajor:    1,
					ProtoMinor:    req1.ProtoMinor,
					Header:        h,
					ContentLength: -1,
				}
				resBuf = bytes.Buffer{}
				res.Write(&resBuf) //nolint:errcheck
				_, err = nconn2.Write(resBuf.Bytes())
				require.NoError(t, err)

				conn := conn.NewConn(bufio.NewReader(base64streamreader.New(buf2)), nconn1)

				req, err2 := conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Options, req.Method)

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Public": base.HeaderValue{strings.Join([]string{
							string(base.Describe),
						}, ", ")},
					},
				})
				require.NoError(t, err2)

				req, err2 = conn.ReadRequest()
				require.NoError(t, err2)
				require.Equal(t, base.Describe, req.Method)
				require.Equal(t, mustParseURL(scheme+"://localhost:8554/teststream"), req.URL)

				medias := []*description.Media{testH264Media}

				err2 = conn.WriteResponse(&base.Response{
					StatusCode: base.StatusOK,
					Header: base.Header{
						"Content-Type": base.HeaderValue{"application/sdp; charset=utf-8"},
						"Content-Base": base.HeaderValue{"/relative-content-base"},
					},
					Body: mediasToSDP(medias),
				})
				require.NoError(t, err2)
			}()

			u, err := base.ParseURL(scheme + "://localhost:8554/teststream")
			require.NoError(t, err)

			c := Client{
				Scheme: u.Scheme,
				Host:   u.Host,
				Tunnel: TunnelHTTP,
				TLSConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}

			err = c.Start()
			require.NoError(t, err)
			defer c.Close()

			_, _, err = c.Describe(u)
			require.NoError(t, err)
		})
	}
}
