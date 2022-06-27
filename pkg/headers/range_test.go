package headers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

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
				Start: RangeNPTTime(123.45 * float64(time.Second)),
				End: func() *RangeNPTTime {
					v := RangeNPTTime(125 * time.Second)
					return &v
				}(),
			},
		},
	},
	{
		"npt open ended",
		base.HeaderValue{`npt=12:05:35.3-`},
		base.HeaderValue{`npt=43535.3-`},
		Range{
			Value: &RangeNPT{
				Start: RangeNPTTime(float64(12*3600+5*60+35.3) * float64(time.Second)),
			},
		},
	},
	{
		"clock",
		base.HeaderValue{`clock=19961108T142300Z-19961108T143520Z`},
		base.HeaderValue{`clock=19961108T142300Z-19961108T143520Z`},
		Range{
			Value: &RangeUTC{
				Start: RangeUTCTime(time.Date(1996, 11, 8, 14, 23, 0, 0, time.UTC)),
				End: func() *RangeUTCTime {
					v := RangeUTCTime(time.Date(1996, 11, 8, 14, 35, 20, 0, time.UTC))
					return &v
				}(),
			},
		},
	},
	{
		"clock open ended",
		base.HeaderValue{`clock=19960213T143205Z-`},
		base.HeaderValue{`clock=19960213T143205Z-`},
		Range{
			Value: &RangeUTC{
				Start: RangeUTCTime(time.Date(1996, 2, 13, 14, 32, 5, 0, time.UTC)),
			},
		},
	},
	{
		"time",
		base.HeaderValue{`clock=19960213T143205Z-;time=19970123T143720Z`},
		base.HeaderValue{`clock=19960213T143205Z-;time=19970123T143720Z`},
		Range{
			Value: &RangeUTC{
				Start: RangeUTCTime(time.Date(1996, 2, 13, 14, 32, 5, 0, time.UTC)),
			},
			Time: func() *RangeUTCTime {
				v := RangeUTCTime(time.Date(1997, 1, 23, 14, 37, 20, 0, time.UTC))
				return &v
			}(),
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

func TestRangeUnmarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		hv   base.HeaderValue
		err  string
	}{
		{
			"empty",
			base.HeaderValue{},
			"value not provided",
		},
		{
			"2 values",
			base.HeaderValue{"a", "b"},
			"value provided multiple times ([a b])",
		},
		{
			"invalid keys",
			base.HeaderValue{`key1="k`},
			"apexes not closed (key1=\"k)",
		},
		{
			"value not found",
			base.HeaderValue{``},
			"value not found ()",
		},
		{
			"smpte without values",
			base.HeaderValue{`smpte=`},
			"invalid value ()",
		},
		{
			"smtpe end invalid",
			base.HeaderValue{`smpte=00:00:01-123`},
			"invalid SMPTE time (123)",
		},
		{
			"smpte invalid 1",
			base.HeaderValue{`smpte=123-`},
			"invalid SMPTE time (123)",
		},
		{
			"smpte invalid 2",
			base.HeaderValue{`smpte=aa:00:00-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"smpte invalid 3",
			base.HeaderValue{`smpte=00:aa:00-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"smpte invalid 4",
			base.HeaderValue{`smpte=00:00:aa-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"smpte invalid 5",
			base.HeaderValue{`smpte=00:00:00:aa-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"smpte invalid 6",
			base.HeaderValue{`smpte=00:00:00:aa.00-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"smpte invalid 7",
			base.HeaderValue{`smpte=00:00:00:00.aa-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"npt without values",
			base.HeaderValue{`npt=`},
			"invalid value ()",
		},
		{
			"npt end invalid",
			base.HeaderValue{`npt=00:00:00-aa`},
			"strconv.ParseFloat: parsing \"aa\": invalid syntax",
		},
		{
			"npt invalid 1",
			base.HeaderValue{`npt=00:00:00:00-`},
			"invalid NPT time (00:00:00:00)",
		},
		{
			"npt invalid 2",
			base.HeaderValue{`npt=aa-`},
			"strconv.ParseFloat: parsing \"aa\": invalid syntax",
		},
		{
			"npt invalid 3",
			base.HeaderValue{`npt=aa:00-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"npt invalid 4",
			base.HeaderValue{`npt=aa:00:00-`},
			"strconv.ParseUint: parsing \"aa\": invalid syntax",
		},
		{
			"clock without values",
			base.HeaderValue{`clock=`},
			"invalid value ()",
		},
		{
			"clock end invalid",
			base.HeaderValue{`clock=20060102T150405Z-aa`},
			"parsing time \"aa\" as \"20060102T150405Z\": cannot parse \"aa\" as \"2006\"",
		},
		{
			"clock invalid 1",
			base.HeaderValue{`clock=aa-`},
			"parsing time \"aa\" as \"20060102T150405Z\": cannot parse \"aa\" as \"2006\"",
		},
		{
			"time invalid",
			base.HeaderValue{`time=aa`},
			"parsing time \"aa\" as \"20060102T150405Z\": cannot parse \"aa\" as \"2006\"",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var h Range
			err := h.Unmarshal(ca.hv)
			require.EqualError(t, err, ca.err)
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
