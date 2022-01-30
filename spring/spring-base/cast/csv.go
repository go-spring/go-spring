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

package cast

import (
	"bytes"
	"errors"
	"strconv"
)

// ToCSV 将数据转换为 CSV 格式，可用于 redis 结果格式化。
func ToCSV(data []interface{}) string {
	var buf bytes.Buffer
	for i, arg := range data {
		switch s := arg.(type) {
		case string:
			if c := QuoteCount(s); c == 1 {
				s = strconv.Quote(s)
			}
			buf.WriteString(strconv.Quote(s))
		default:
			buf.WriteString(strconv.Quote(ToString(arg)))
		}
		if i < len(data)-1 {
			buf.WriteByte(',')
		}
	}
	return buf.String()
}

// ParseCSV 将 CSV 格式的数据转换为字符串数组。
func ParseCSV(data string) ([]string, error) {
	var (
		ret []string
		buf bytes.Buffer
	)
	for i := 0; ; {
		if i >= len(data) {
			return ret, nil
		}
		buf.Reset()
		var (
			done          bool
			inQuote       bool
			inSingleQuote bool
		)
		for ; !done; i++ {
			if i >= len(data) && (inQuote || inSingleQuote) {
				return nil, errors.New("invalid syntax")
			}
			if c := data[i]; inQuote {
				if c == '\\' && i < len(data)-3 && data[i+1] == 'x' && IsHexDigit(data[i+2]) && IsHexDigit(data[i+3]) {
					b1 := HexDigitToInt(data[i+2]) * 16
					b2 := HexDigitToInt(data[i+3])
					b := byte(b1 + b2)
					buf.WriteByte(b)
					i += 3
				} else if c == '\\' && i < len(data)-1 {
					i++
					switch c = data[i]; c {
					case 'n':
						c = '\n'
					case 'r':
						c = '\r'
					case 't':
						c = '\t'
					case 'b':
						c = '\b'
					case 'a':
						c = '\a'
					}
					buf.WriteByte(c)
				} else if c == '"' {
					done = true
				} else {
					buf.WriteByte(c)
				}
			} else if inSingleQuote {
				if c == '\\' && i < len(data)-1 && data[i+1] == '\'' {
					i++
					buf.WriteByte('\'')
				} else if c == '\'' {
					done = true
				} else {
					buf.WriteByte(c)
				}
			} else {
				switch c {
				case ',':
					if inQuote {
						return nil, errors.New("invalid syntax")
					}
				case '"':
					inQuote = true
				case '\'':
					inSingleQuote = true
				default:
					buf.WriteByte(c)
				}
				if i == len(data)-1 {
					done = true
				}
			}
		}
		ret = append(ret, buf.String())
	}
}
