package base

import (
    "net/url"
    "testing"

    "github.com/stretchr/testify/require"
)

func TestURLGetBasePath(t *testing.T) {
    for _, ca := range []struct{
        u *url.URL
        b string
    } {
        {
            urlMustParse("rtsp://localhost:8554/teststream"),
            "teststream",
        },
        {
            urlMustParse("rtsp://localhost:8554/test/stream"),
            "test/stream",
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp"),
            "test",
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
            "te!st",
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
            "user=tmp&password=BagRep1!&channel=1&stream=0.sdp",
        },
    } {
        b := URLGetBasePath(ca.u)
        require.Equal(t, ca.b, b)
    }
}

func TestURLGetBaseControlPath(t *testing.T) {
    for _, ca := range []struct{
        u *url.URL
        b string
        c string
    } {
        {
            urlMustParse("rtsp://localhost:8554/teststream/trackID=1"),
            "teststream",
            "trackID=1",
        },
        {
            urlMustParse("rtsp://localhost:8554/test/stream/trackID=1"),
            "test/stream",
            "trackID=1",
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1"),
            "test",
            "trackID=1",
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
            "te!st",
            "trackID=1",
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
            "user=tmp&password=BagRep1!&channel=1&stream=0.sdp",
            "trackID=1",
        },
    } {
        b, c, ok := URLGetBaseControlPath(ca.u)
        require.Equal(t, true, ok)
        require.Equal(t, ca.b, b)
        require.Equal(t, ca.c, c)
    }
}

func TestURLAddControlPath(t *testing.T) {
    for _, ca := range []struct{
        u *url.URL
        ou *url.URL
    } {
        {
            urlMustParse("rtsp://localhost:8554/teststream"),
            urlMustParse("rtsp://localhost:8554/teststream/trackID=1"),
        },
        {
            urlMustParse("rtsp://localhost:8554/test/stream"),
            urlMustParse("rtsp://localhost:8554/test/stream/trackID=1"),
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp"),
            urlMustParse("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1"),
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
            urlMustParse("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
        },
        {
            urlMustParse("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
            urlMustParse("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
        },
    } {
        URLAddControlPath(ca.u, "trackID=1")
        require.Equal(t, ca.ou, ca.u)
    }
}
