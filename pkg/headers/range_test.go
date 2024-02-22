package headers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

func durationPtr(v time.Duration) *time.Duration {
	return &v
}

func timePtr(v time.Time) *time.Time {
	return &v
}

var casesRange = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    Range
}{
	{
		"smpte",
		base.HeaderValue{`smpte=10:07:00-10:07:33:05.01`},
		base.HeaderValue{`smpte=10:07:00-10:07:33:05.01`},
		Range{
			Value: &RangeSMPTE{
				Start: RangeSMPTETime{
					Time: time.Duration(7*60+10*3600) * time.Second,
				},
				End: &RangeSMPTETime{
					Time:     time.Duration(33+7*60+10*3600) * time.Second,
					Frame:    5,
					Subframe: 1,
				},
			},
		},
	},
	{
		"smpte open ended",
		base.HeaderValue{`smpte=0:10:00-`},
		base.HeaderValue{`smpte=0:10:00-`},
		Range{
			Value: &RangeSMPTE{
				Start: RangeSMPTETime{
					Time: time.Duration(10*60) * time.Second,
				},
			},
		},
	},
	{
		"smpte with frame",
		base.HeaderValue{`smpte=0:10:00:01-`},
		base.HeaderValue{`smpte=0:10:00:01-`},
		Range{
			Value: &RangeSMPTE{
				Start: RangeSMPTETime{
					Time:  time.Duration(10*60) * time.Second,
					Frame: 1,
				},
			},
		},
	},
	{
		"npt",
		base.HeaderValue{`npt=123.45-125`},
		base.HeaderValue{`npt=123.45-125`},
		Range{
			Value: &RangeNPT{
				Start: time.Duration(123.45 * float64(time.Second)),
				End:   durationPtr(125 * time.Second),
			},
		},
	},
	{
		"npt open ended",
		base.HeaderValue{`npt=12:05:35.3-`},
		base.HeaderValue{`npt=43535.3-`},
		Range{
			Value: &RangeNPT{
				Start: time.Duration(float64(12*3600+5*60+35.3) * float64(time.Second)),
			},
		},
	},
	{
		"clock",
		base.HeaderValue{`clock=19961108T142300Z-19961108T143520Z`},
		base.HeaderValue{`clock=19961108T142300Z-19961108T143520Z`},
		Range{
			Value: &RangeUTC{
				Start: time.Date(1996, 11, 8, 14, 23, 0, 0, time.UTC),
				End:   timePtr(time.Date(1996, 11, 8, 14, 35, 20, 0, time.UTC)),
			},
		},
	},
	{
		"clock open ended",
		base.HeaderValue{`clock=19960213T143205Z-`},
		base.HeaderValue{`clock=19960213T143205Z-`},
		Range{
			Value: &RangeUTC{
				Start: time.Date(1996, 2, 13, 14, 32, 5, 0, time.UTC),
			},
		},
	},
	{
		"time",
		base.HeaderValue{`clock=19960213T143205Z-;time=19970123T143720Z`},
		base.HeaderValue{`clock=19960213T143205Z-;time=19970123T143720Z`},
		Range{
			Value: &RangeUTC{
				Start: time.Date(1996, 2, 13, 14, 32, 5, 0, time.UTC),
			},
			Time: timePtr(time.Date(1997, 1, 23, 14, 37, 20, 0, time.UTC)),
		},
	},
}

func TestRangeUnmarshal(t *testing.T) {
	for _, ca := range casesRange {
		t.Run(ca.name, func(t *testing.T) {
			var h Range
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestRangeMarshal(t *testing.T) {
	for _, ca := range casesRange {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Marshal()
			require.Equal(t, ca.vout, req)
		})
	}
}

func FuzzRangeUnmarshal(f *testing.F) {
	for _, ca := range casesRange {
		f.Add(ca.vin[0])
	}

	f.Add("smtpe=")
	f.Add("npt=")
	f.Add("clock=")

	f.Fuzz(func(t *testing.T, b string) {
		var h Range
		h.Unmarshal(base.HeaderValue{b}) //nolint:errcheck
	})
}

func TestRangeAdditionalErrors(t *testing.T) {
	func() {
		var h Range
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h Range
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
