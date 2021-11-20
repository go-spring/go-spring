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

// maps
var standards = map[byte]string{
	'd': "02",
	'D': "Mon",
	'j': "1",
	'Y': "2006",
	'y': "06",
	'm': "01",
	'M': "Jan",
	'a': "pm",
	'A': "PM",
	'H': "15",
	'h': "3",
	'i': "04",
	's': "05",
}

// Format time 支持 YY-MM-DD 格式化会不会更好呢？
func Format(t time.Time, layout string) string {
	layout = toStandardLayout(layout)
	return t.Format(layout)
}

// toStandardLayout
func toStandardLayout(format string) string {
	buf := bytes.NewBuffer(nil)
	for i := 0; i < len(format); i++ {
		if layout, ok := standards[format[i]]; ok {
			buf.WriteString(layout)
		} else {
			buf.WriteByte(format[i])
		}
	}
	return buf.String()
}
