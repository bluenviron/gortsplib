package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

func TestClientTunnelHTTPRequestURL(t *testing.T) {
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
		{
			name:           "leading double slash",
			streamURL:      mustParseURL("rtsp://localhost:8554//teststream"),
			expectedTarget: "//teststream",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expectedTarget, clientTunnelHTTPRequestURL(testCase.streamURL).RequestURI())
		})
	}
}
