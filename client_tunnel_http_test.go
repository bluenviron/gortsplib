package gortsplib

import (
	"bufio"
	"context"
	"crypto/md5"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
)

func TestClientTunnelHTTPRequestTarget(t *testing.T) {
	for _, testCase := range []struct {
		name           string
		streamURL      *base.URL
		expectedTarget string
	}{
		{
			name:           "nil url",
			streamURL:      nil,
			expectedTarget: "/",
		},
		{
			name:           "empty path",
			streamURL:      mustParseURL("rtsp://localhost:8554"),
			expectedTarget: "/",
		},
		{
			name:           "path with query",
			streamURL:      mustParseURL("rtsp://localhost:8554/teststream?param=value"),
			expectedTarget: "/teststream?param=value",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expectedTarget, clientTunnelHTTPRequestTarget(testCase.streamURL))
		})
	}
}

func md5HexTest(in string) string {
	h := md5.Sum([]byte(in))
	return hex.EncodeToString(h[:])
}

// tunnelAuthServer is a RTSP-over-HTTP tunnel endpoint that requires HTTP
// authentication on the tunnel requests, like AXIS OS 13 does.
type tunnelAuthServer struct {
	ln        net.Listener
	challenge string
	rejectAll bool
	mutex     sync.Mutex
	requests  []*http.Request
}

func newTunnelAuthServer(t *testing.T, challenge string) *tunnelAuthServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &tunnelAuthServer{ln: ln, challenge: challenge}
	t.Cleanup(func() { ln.Close() })

	go func() {
		for {
			nconn, err2 := ln.Accept()
			if err2 != nil {
				return
			}
			go s.handle(nconn)
		}
	}()

	return s
}

func (s *tunnelAuthServer) handle(nconn net.Conn) {
	defer nconn.Close()

	req, err := http.ReadRequest(bufio.NewReader(nconn))
	if err != nil {
		return
	}

	s.mutex.Lock()
	s.requests = append(s.requests, req)
	s.mutex.Unlock()

	if req.Header.Get("Authorization") == "" || s.rejectAll {
		nconn.Write([]byte("HTTP/1.1 401 Unauthorized\r\n" + //nolint:errcheck
			"WWW-Authenticate: " + s.challenge + "\r\n" +
			"Content-Length: 0\r\n" +
			"Connection: close\r\n" +
			"\r\n"))
		return
	}

	if req.Method == http.MethodGet {
		nconn.Write([]byte("HTTP/1.0 200 OK\r\n" + //nolint:errcheck
			"Content-Type: application/x-rtsp-tunnelled\r\n" +
			"\r\n"))
	}

	// keep the tunnel channel open until the client closes it.
	buf := make([]byte, 1024)
	for {
		_, err = nconn.Read(buf)
		if err != nil {
			return
		}
	}
}

func (s *tunnelAuthServer) authorizations(method string) []string {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var out []string
	for _, req := range s.requests {
		if req.Method == method {
			out = append(out, req.Header.Get("Authorization"))
		}
	}
	return out
}

func TestClientTunnelHTTPAuthentication(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		s := newTunnelAuthServer(t, `Basic realm="testrealm"`)

		u := mustParseURL("rtsp://myuser:mypass@" + s.ln.Addr().String() + "/axis-media/media.amp?camera=1")
		nconn, err := newClientTunnelHTTP(context.Background(), s.ln.Addr().String(), false, nil,
			(&net.Dialer{}).DialContext, nil, u)
		require.NoError(t, err)
		defer nconn.Close()

		// the anonymous GET got the challenge, the second one carried the credentials.
		require.Equal(t, []string{"", "Basic bXl1c2VyOm15cGFzcw=="}, s.authorizations(http.MethodGet))
		require.Eventually(t, func() bool {
			return len(s.authorizations(http.MethodPost)) == 1
		}, 2e9, 1e7)
		require.Equal(t, []string{"Basic bXl1c2VyOm15cGFzcw=="}, s.authorizations(http.MethodPost))
	})

	t.Run("digest", func(t *testing.T) {
		s := newTunnelAuthServer(t, `Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce", qop="auth"`)

		u := mustParseURL("rtsp://myuser:mypass@" + s.ln.Addr().String() + "/axis-media/media.amp?camera=1")
		nconn, err := newClientTunnelHTTP(context.Background(), s.ln.Addr().String(), false, nil,
			(&net.Dialer{}).DialContext, nil, u)
		require.NoError(t, err)
		defer nconn.Close()

		require.Eventually(t, func() bool {
			return len(s.authorizations(http.MethodPost)) == 1
		}, 2e9, 1e7)

		// the digest response covers the method and the request target of each
		// tunnel request, with an increasing nonce count.
		for i, check := range []struct {
			method string
			value  string
			nc     string
		}{
			{http.MethodGet, s.authorizations(http.MethodGet)[1], "00000001"},
			{http.MethodPost, s.authorizations(http.MethodPost)[0], "00000002"},
		} {
			var h headers.Authorization
			require.NoError(t, h.Unmarshal(base.HeaderValue{check.value}), i)
			require.Equal(t, headers.AuthMethodDigest, h.Method)
			require.Equal(t, "/axis-media/media.amp?camera=1", h.URI)
			require.Equal(t, check.nc, *h.Nc)
			ha1 := md5HexTest("myuser:myrealm:mypass")
			ha2 := md5HexTest(check.method + ":" + h.URI)
			require.Equal(t, md5HexTest(ha1+":"+h.Nonce+":"+*h.Nc+":"+*h.Cnonce+":auth:"+ha2), h.Response)
		}
	})

	t.Run("no credentials", func(t *testing.T) {
		s := newTunnelAuthServer(t, `Basic realm="testrealm"`)

		u := mustParseURL("rtsp://" + s.ln.Addr().String() + "/axis-media/media.amp")
		_, err := newClientTunnelHTTP(context.Background(), s.ln.Addr().String(), false, nil,
			(&net.Dialer{}).DialContext, nil, u)
		require.EqualError(t, err, "bad status code: 401")
		require.Len(t, s.authorizations(http.MethodGet), 1)
	})

	t.Run("rejected credentials", func(t *testing.T) {
		s := newTunnelAuthServer(t, `Basic realm="testrealm"`)
		s.rejectAll = true

		u := mustParseURL("rtsp://myuser:wrongpass@" + s.ln.Addr().String() + "/axis-media/media.amp")
		_, err := newClientTunnelHTTP(context.Background(), s.ln.Addr().String(), false, nil,
			(&net.Dialer{}).DialContext, nil, u)
		require.EqualError(t, err, "bad status code: 401")

		// one anonymous request, one authenticated request, no loop.
		gets := s.authorizations(http.MethodGet)
		require.Len(t, gets, 2)
		require.Equal(t, "", gets[0])
		require.True(t, strings.HasPrefix(gets[1], "Basic "))
	})
}
