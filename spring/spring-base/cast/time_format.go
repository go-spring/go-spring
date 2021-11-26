package cast

import (
	"fmt"
	"time"
)

type timeFormatType int

const (
	timeFormatNoTimezone timeFormatType = iota
	timeFormatNamedTimezone
	timeFormatNumericTimezone
	timeFormatNumericAndNamedTimezone
	timeFormatTimeOnly
)

type timeFormat struct {
	format string
	typ    timeFormatType
}

func (f timeFormat) hasTimezone() bool {
	// We don't include the formats with only named timezones, see
	// https://github.com/golang/go/issues/19694#issuecomment-289103522
	return f.typ >= timeFormatNumericTimezone && f.typ <= timeFormatNumericAndNamedTimezone
}

var (
	timeFormats = []timeFormat{
		timeFormat{time.RFC3339, timeFormatNumericTimezone},
		timeFormat{"2006-01-02T15:04:05", timeFormatNoTimezone}, // iso8601 without timezone
		timeFormat{time.RFC1123Z, timeFormatNumericTimezone},
		timeFormat{time.RFC1123, timeFormatNamedTimezone},
		timeFormat{time.RFC822Z, timeFormatNumericTimezone},
		timeFormat{time.RFC822, timeFormatNamedTimezone},
		timeFormat{time.RFC850, timeFormatNamedTimezone},
		timeFormat{"2006-01-02 15:04:05.999999999 -0700 MST", timeFormatNumericAndNamedTimezone}, // Time.String()
		timeFormat{"2006-01-02T15:04:05-0700", timeFormatNumericTimezone},                        // RFC3339 without timezone hh:mm colon
		timeFormat{"2006-01-02 15:04:05Z0700", timeFormatNumericTimezone},                        // RFC3339 without T or timezone hh:mm colon
		timeFormat{"2006-01-02 15:04:05", timeFormatNoTimezone},
		timeFormat{time.ANSIC, timeFormatNoTimezone},
		timeFormat{time.UnixDate, timeFormatNamedTimezone},
		timeFormat{time.RubyDate, timeFormatNumericTimezone},
		timeFormat{"2006-01-02 15:04:05Z07:00", timeFormatNumericTimezone},
		timeFormat{"2006-01-02", timeFormatNoTimezone},
		timeFormat{"02 Jan 2006", timeFormatNoTimezone},
		timeFormat{"2006-01-02 15:04:05 -07:00", timeFormatNumericTimezone},
		timeFormat{"2006-01-02 15:04:05 -0700", timeFormatNumericTimezone},
		timeFormat{time.Kitchen, timeFormatTimeOnly},
		timeFormat{time.Stamp, timeFormatTimeOnly},
		timeFormat{time.StampMilli, timeFormatTimeOnly},
		timeFormat{time.StampMicro, timeFormatTimeOnly},
		timeFormat{time.StampNano, timeFormatTimeOnly},
	}
)

func parseDateWith(s string, location *time.Location) (d time.Time, e error) {
	for _, format := range timeFormats {
		if d, e = time.Parse(format.format, s); e == nil {
			// Some time formats have a zone name, but no offset, so it gets
			// put in that zone name (not the default one passed in to us), but
			// without that zone's offset. So set the location manually.
			if format.typ <= timeFormatNamedTimezone {
				if location == nil {
					location = time.Local
				}
				year, month, day := d.Date()
				hour, min, sec := d.Clock()
				d = time.Date(year, month, day, hour, min, sec, d.Nanosecond(), location)
			}

			return
		}
	}
	return d, fmt.Errorf("unable to parse date: %s", s)
}
