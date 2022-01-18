package rtptimedec

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOverflow(t *testing.T) {
	d := New(90000)

	pts := d.Decode(4294877295)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(90000)
	require.Equal(t, 2*time.Second, pts)
}

func TestOverflowAndBack(t *testing.T) {
	d := New(90000)

	pts := d.Decode(4294877296)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(90001)
	require.Equal(t, 2*time.Second, pts)

	pts = d.Decode(4294877296)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(4294877296 - 90000)
	require.Equal(t, -1*time.Second, pts)

	pts = d.Decode(4294877296)
	require.Equal(t, time.Duration(0), pts)

	pts = d.Decode(90001)
	require.Equal(t, 2*time.Second, pts)
}
