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

package recorder

import (
	"bytes"
	"errors"
	"strconv"
	"unicode"
	"unicode/utf8"

	"github.com/go-spring/spring-base/cast"
)

// csvQuoteCount 查询字符串需要 quote 的次数，无需 quote 返回 0，
// 包含引号 " 返回 1，包含非法的 unicode 字符返回 2。
func csvQuoteCount(s string) int {
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b == '"' {
				return 1
			}
			i++
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			return 2
		}
		i += size
	}
	return 0
}

// EncodeCSV 将数据转换为 CSV 格式，可用于 redis 结果格式化。
func EncodeCSV(data ...interface{}) string {
	var buf bytes.Buffer
	for i, arg := range data {
		switch s := arg.(type) {
		case nil:
			buf.WriteString("NULL")
		case []interface{}:
			return EncodeCSV(s...)
		case string:
			if c := csvQuoteCount(s); c == 1 {
				s = strconv.Quote(s)
			}
			buf.WriteString(strconv.Quote(s))
		default:
			buf.WriteString(strconv.Quote(cast.ToString(arg)))
		}
		if i < len(data)-1 {
			buf.WriteByte(',')
		}
	}
	return buf.String()
}

// DecodeCSV 将 CSV 格式的数据转换为字符串数组。
func DecodeCSV(data string) ([]string, error) {
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
				if c == '\\' && i < len(data)-3 && data[i+1] == 'x' && cast.IsHexDigit(data[i+2]) && cast.IsHexDigit(data[i+3]) {
					b1 := cast.HexDigitToInt(data[i+2]) * 16
					b2 := cast.HexDigitToInt(data[i+3])
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

// ttyQuoteCount 查询字符串需要 quote 的次数，无需 quote 返回 0，
// 包含引号及空格等返回 1，包含非法的 unicode 字符返回 2。
func ttyQuoteCount(s string) int {
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			switch b {
			case '"', '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
				return 1
			}
			i++
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			return 2
		}
		i += size
	}
	return 0
}

// EncodeTTY 将数据转换为命令行格式，可用于 redis 参数格式化。
func EncodeTTY(data ...interface{}) string {
	var buf bytes.Buffer
	for i, arg := range data {
		switch s := arg.(type) {
		case string:
			if c := ttyQuoteCount(s); c > 0 {
				s = strconv.Quote(s)
			}
			buf.WriteString(s)
		default:
			buf.WriteString(cast.ToString(arg))
		}
		if i < len(data)-1 {
			buf.WriteByte(' ')
		}
	}
	return buf.String()
}

// DecodeTTY 将命令行格式的数据转换为字符串数组。
func DecodeTTY(data string) ([]string, error) {
	var (
		ret []string
		buf bytes.Buffer
	)
	for i := 0; ; {
		for i < len(data) && unicode.IsSpace(rune(data[i])) {
			i++
		}
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
				if c == '\\' && i < len(data)-3 && data[i+1] == 'x' && cast.IsHexDigit(data[i+2]) && cast.IsHexDigit(data[i+3]) {
					b1 := cast.HexDigitToInt(data[i+2]) * 16
					b2 := cast.HexDigitToInt(data[i+3])
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
				case ' ':
					fallthrough
				case '\n':
					fallthrough
				case '\r':
					fallthrough
				case '\t':
					done = true
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
