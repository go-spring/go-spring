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

package util

import (
	"bytes"
	"time"
)

// Format time 支持 YY-MM-DD 格式化会不会更好呢？
func Format(t time.Time, layout string) string {
	layout = ToNativeLayout(layout)
	return t.Format(layout)
}

// ToNativeLayout timestamp convert 2006/01/02 15:04:05
func ToNativeLayout(layout string) string {
	buf := bytes.NewBuffer(nil)

	for layout != "" {
		prefix, std, suffix := nextChunk(layout)
		if prefix != "" {
			buf.WriteString(prefix)
		}
		layout = suffix
		if std != "" {
			buf.WriteString(std)
		}
	}

	return buf.String()
}

// nextChunk
func nextChunk(layout string) (prefix string, now string, suffix string) {
	for i := 0; i < len(layout); i++ {
		switch b := layout[i]; b {
		case 'y': // yy yyyy
			if len(layout) >= i+4 && layout[i:i+4] == "yyyy" {
				return layout[0:i], "2006", layout[i+4:]
			}
			if len(layout) >= i+2 && layout[i:i+2] == "yy" {
				return layout[0:i], "06", layout[i+2:]
			}
		case 'M': // MM MMM
			if len(layout) >= i+3 && layout[i:i+3] == "MMM" {
				return layout[0:i], "Jan", layout[i+3:]
			}
			if len(layout) >= i+2 && layout[i:i+2] == "MM" {
				return layout[0:i], "01", layout[i+2:]
			}
		case 'd': // dd
			if len(layout) >= i+2 && layout[i:i+2] == "dd" {
				return layout[0:i], "02", layout[i+2:]
			}
		case 'D': // d
			if len(layout) >= i+1 && layout[i:i+1] == "D" {
				return layout[0:i], "002", layout[i+1:]
			}
		case 'H': // H
			if len(layout) >= i+1 && layout[i:i+1] == "H" {
				return layout[0:i], "15", layout[i+1:]
			}
		case 'h': // h
			if len(layout) >= i+1 && layout[i:i+1] == "h" {
				return layout[0:i], "03", layout[i+1:]
			}
		case 'm': // m
			if len(layout) >= i+1 && layout[i:i+1] == "m" {
				return layout[0:i], "04", layout[i+1:]
			}
		case 's': // s
			if len(layout) >= i+1 && layout[i:i+1] == "s" {
				return layout[0:i], "05", layout[i+1:]
			}
		}
	}

	return layout, "", ""
}
