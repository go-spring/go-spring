/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package clock

import (
	"bytes"
	"time"
)

const (
	stdNone        = ""
	stdLongYear    = "2006"
	stdYear        = "06"
	stdMonth       = "Jan"
	stdZeroMonth   = "01"
	stdZeroDay     = "02"
	stdZeroYearDay = "002"
	stdHour        = "15"
	stdZeroHour12  = "03"
	stdZeroMinute  = "04"
	stdZeroSecond  = "05"
)

// Format returns a textual representation of the time value formatted
// according to layout.
func Format(t time.Time, layout string) string {
	layout = ToStdLayout(layout)
	if layout == "" {
		return ""
	}
	return t.Format(layout)
}

// ToStdLayout converts "yyyy-MM-dd H:m:s" to "2006-01-02 15:04:05".
func ToStdLayout(layout string) string {
	buf := bytes.NewBuffer(nil)
	for layout != "" {
		prefix, std, suffix := nextStdChunk(layout)
		if prefix != "" {
			buf.WriteString(prefix)
		}
		if std != "" {
			buf.WriteString(std)
		}
		layout = suffix
	}
	return buf.String()
}

func nextStdChunk(layout string) (prefix string, std string, suffix string) {
	for i := 0; i < len(layout); i++ {
		switch b := layout[i]; b {
		case 'y': // yy yyyy
			if len(layout) >= i+4 && layout[i:i+4] == "yyyy" {
				return layout[0:i], stdLongYear, layout[i+4:]
			}
			if len(layout) >= i+2 && layout[i:i+2] == "yy" {
				return layout[0:i], stdYear, layout[i+2:]
			}
		case 'M': // MM MMM
			if len(layout) >= i+3 && layout[i:i+3] == "MMM" {
				return layout[0:i], stdMonth, layout[i+3:]
			}
			if len(layout) >= i+2 && layout[i:i+2] == "MM" {
				return layout[0:i], stdZeroMonth, layout[i+2:]
			}
		case 'd': // dd
			if len(layout) >= i+2 && layout[i:i+2] == "dd" {
				return layout[0:i], stdZeroDay, layout[i+2:]
			}
		case 'D': // d
			if len(layout) >= i+1 && layout[i:i+1] == "D" {
				return layout[0:i], stdZeroYearDay, layout[i+1:]
			}
		case 'H': // H
			if len(layout) >= i+1 && layout[i:i+1] == "H" {
				return layout[0:i], stdHour, layout[i+1:]
			}
		case 'h': // h
			if len(layout) >= i+1 && layout[i:i+1] == "h" {
				return layout[0:i], stdZeroHour12, layout[i+1:]
			}
		case 'm': // m
			if len(layout) >= i+1 && layout[i:i+1] == "m" {
				return layout[0:i], stdZeroMinute, layout[i+1:]
			}
		case 's': // s
			if len(layout) >= i+1 && layout[i:i+1] == "s" {
				return layout[0:i], stdZeroSecond, layout[i+1:]
			}
		}
	}
	return layout, stdNone, ""
}
