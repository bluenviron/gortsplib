package headers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
)

func leadingZero(v uint) string {
	ret := ""
	if v < 10 {
		ret += "0"
	}
	ret += strconv.FormatUint(uint64(v), 10)
	return ret
}

// RangeSMPTETime is a time expressed in SMPTE unit.
type RangeSMPTETime struct {
	Time     time.Duration
	Frame    uint
	Subframe uint
}

func (t *RangeSMPTETime) read(s string) error {
	parts := strings.Split(s, ":")
	if len(parts) != 3 && len(parts) != 4 {
		return fmt.Errorf("invalid SMPTE time (%v)", s)
	}

	tmp, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		return err
	}
	hours := tmp

	tmp, err = strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return err
	}
	mins := tmp

	tmp, err = strconv.ParseUint(parts[2], 10, 64)
	if err != nil {
		return err
	}
	seconds := tmp

	t.Time = time.Duration(seconds+mins*60+hours*3600) * time.Second

	if len(parts) == 4 {
		parts := strings.Split(parts[3], ".")
		if len(parts) == 2 {
			tmp, err := strconv.ParseUint(parts[0], 10, 64)
			if err != nil {
				return err
			}
			t.Frame = uint(tmp)

			tmp, err = strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				return err
			}
			t.Subframe = uint(tmp)

		} else {
			tmp, err := strconv.ParseUint(parts[0], 10, 64)
			if err != nil {
				return err
			}
			t.Frame = uint(tmp)
		}
	}

	return nil
}

func (t RangeSMPTETime) write() string {
	d := uint64(t.Time.Seconds())
	hours := d / 3600
	d %= 3600
	mins := d / 60
	secs := d % 60

	ret := strconv.FormatUint(hours, 10) + ":" + leadingZero(uint(mins)) + ":" + leadingZero(uint(secs))

	if t.Frame > 0 || t.Subframe > 0 {
		ret += ":" + leadingZero(t.Frame)

		if t.Subframe > 0 {
			ret += "." + leadingZero(t.Subframe)
		}
	}

	return ret
}

// RangeSMPTE is a range expressed in SMPTE unit.
type RangeSMPTE struct {
	Start RangeSMPTETime
	End   *RangeSMPTETime
}

func (r *RangeSMPTE) read(start string, end string) error {
	err := r.Start.read(start)
	if err != nil {
		return err
	}

	if end != "" {
		var v RangeSMPTETime
		err := v.read(end)
		if err != nil {
			return err
		}
		r.End = &v
	}

	return nil
}

func (r RangeSMPTE) write() string {
	ret := "smpte=" + r.Start.write() + "-"
	if r.End != nil {
		ret += r.End.write()
	}
	return ret
}

// RangeNPTTime is a time expressed in NPT unit.
type RangeNPTTime time.Duration

func (t *RangeNPTTime) read(s string) error {
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return fmt.Errorf("invalid NPT time (%v)", s)
	}

	var hours uint64
	if len(parts) == 3 {
		tmp, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return err
		}
		hours = tmp
		parts = parts[1:]
	}

	var mins uint64
	if len(parts) >= 2 {
		tmp, err := strconv.ParseUint(parts[0], 10, 64)
		if err != nil {
			return err
		}
		mins = tmp
		parts = parts[1:]
	}

	tmp, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return err
	}
	seconds := tmp

	*t = RangeNPTTime(time.Duration(seconds*float64(time.Second)) +
		time.Duration(mins*60+hours*3600)*time.Second)

	return nil
}

func (t RangeNPTTime) write() string {
	return strconv.FormatFloat(time.Duration(t).Seconds(), 'f', -1, 64)
}

// RangeNPT is a range expressed in NPT unit.
type RangeNPT struct {
	Start RangeNPTTime
	End   *RangeNPTTime
}

func (r *RangeNPT) read(start string, end string) error {
	err := r.Start.read(start)
	if err != nil {
		return err
	}

	if end != "" {
		var v RangeNPTTime
		err := v.read(end)
		if err != nil {
			return err
		}
		r.End = &v
	}

	return nil
}

func (r RangeNPT) write() string {
	ret := "npt=" + r.Start.write() + "-"
	if r.End != nil {
		ret += r.End.write()
	}
	return ret
}

// RangeUTCTime is a time expressed in UTC unit.
type RangeUTCTime time.Time

func (t *RangeUTCTime) read(s string) error {
	tmp, err := time.Parse("20060102T150405Z", s)
	if err != nil {
		return err
	}

	*t = RangeUTCTime(tmp)
	return nil
}

func (t RangeUTCTime) write() string {
	return time.Time(t).Format("20060102T150405Z")
}

// RangeUTC is a range expressed in UTC unit.
type RangeUTC struct {
	Start RangeUTCTime
	End   *RangeUTCTime
}

func (r *RangeUTC) read(start string, end string) error {
	err := r.Start.read(start)
	if err != nil {
		return err
	}

	if end != "" {
		var v RangeUTCTime
		err := v.read(end)
		if err != nil {
			return err
		}
		r.End = &v
	}

	return nil
}

func (r RangeUTC) write() string {
	ret := "clock=" + r.Start.write() + "-"
	if r.End != nil {
		ret += r.End.write()
	}
	return ret
}

// RangeValue can be
// - RangeSMPTE
// - RangeNPT
// - RangeUTC
type RangeValue interface {
	read(string, string) error
	write() string
}

func rangeValueRead(s RangeValue, v string) error {
	parts := strings.Split(v, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid value (%v)", v)
	}

	return s.read(parts[0], parts[1])
}

// Range is a Range header.
type Range struct {
	// range expressed in a certain unit.
	Value RangeValue

	// time at which the operation is to be made effective.
	Time *RangeUTCTime
}

// Read decodes a Range header.
func (h *Range) Read(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	v0 := v[0]

	kvs, err := keyValParse(v0, ';')
	if err != nil {
		return err
	}

	specFound := false

	for k, v := range kvs {
		switch k {
		case "smpte":
			s := &RangeSMPTE{}
			err := rangeValueRead(s, v)
			if err != nil {
				return err
			}

			specFound = true
			h.Value = s

		case "npt":
			s := &RangeNPT{}
			err := rangeValueRead(s, v)
			if err != nil {
				return err
			}

			specFound = true
			h.Value = s

		case "clock":
			s := &RangeUTC{}
			err := rangeValueRead(s, v)
			if err != nil {
				return err
			}

			specFound = true
			h.Value = s

		case "time":
			t := &RangeUTCTime{}
			err := t.read(v)
			if err != nil {
				return err
			}

			h.Time = t
		}
	}

	if !specFound {
		return fmt.Errorf("value not found (%v)", v[0])
	}

	return nil
}

// Write encodes a Range header.
func (h Range) Write() base.HeaderValue {
	v := h.Value.write()
	if h.Time != nil {
		v += ";time=" + h.Time.write()
	}
	return base.HeaderValue{v}
}
