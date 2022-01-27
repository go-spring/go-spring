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

package replayer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-spring/spring-base/cast"
)

type Message struct {
	data string
}

func (msg *Message) UnmarshalJSON(data []byte) error {
	if data[0] != '"' {
		msg.data = string(data)
		return nil
	}
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	msg.data = s
	return nil
}

func (msg *Message) ToValue(i interface{}) error {
	if strings.HasPrefix(msg.data, "@\"") {
		s, err := strconv.Unquote(msg.data[1:])
		if err != nil {
			return err
		}
		v := reflect.ValueOf(i).Elem()
		switch i.(type) {
		case *string:
			v.Set(reflect.ValueOf(s))
			return nil
		case *[]byte:
			v.Set(reflect.ValueOf([]byte(s)))
			return nil
		default:
			return fmt.Errorf("expect *string or *[]byte but %T", i)
		}
	}
	return json.Unmarshal([]byte(msg.data), i)
}

func (msg *Message) ToCommandLine() ([]string, error) {
	var (
		ret []string
		buf bytes.Buffer
	)
	for i := 0; ; {
		for i < len(msg.data) && unicode.IsSpace(rune(msg.data[i])) {
			i++
		}
		if i >= len(msg.data) {
			return ret, nil
		}
		buf.Reset()
		var (
			done          bool
			inQuote       bool
			inSingleQuote bool
		)
		for ; !done; i++ {
			if i >= len(msg.data) && (inQuote || inSingleQuote) {
				return nil, errors.New("invalid syntax")
			}
			if c := msg.data[i]; inQuote {
				if c == '\\' && i < len(msg.data)-3 && msg.data[i+1] == 'x' && cast.IsHexDigit(msg.data[i+2]) && cast.IsHexDigit(msg.data[i+3]) {
					b1 := cast.HexDigitToInt(msg.data[i+2]) * 16
					b2 := cast.HexDigitToInt(msg.data[i+3])
					b := byte(b1 + b2)
					buf.WriteByte(b)
					i += 3
				} else if c == '\\' && i < len(msg.data)-1 {
					i++
					switch c = msg.data[i]; c {
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
				if c == '\\' && i < len(msg.data)-1 && msg.data[i+1] == '\'' {
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
				if i == len(msg.data)-1 {
					done = true
				}
			}
		}
		ret = append(ret, buf.String())
	}
}

func (msg *Message) ToCSV() ([]string, error) {
	var (
		ret []string
		buf bytes.Buffer
	)
	for i := 0; ; {
		if i >= len(msg.data) {
			return ret, nil
		}
		buf.Reset()
		var (
			done          bool
			inQuote       bool
			inSingleQuote bool
		)
		for ; !done; i++ {
			if i >= len(msg.data) && (inQuote || inSingleQuote) {
				return nil, errors.New("invalid syntax")
			}
			if c := msg.data[i]; inQuote {
				if c == '\\' && i < len(msg.data)-3 && msg.data[i+1] == 'x' && cast.IsHexDigit(msg.data[i+2]) && cast.IsHexDigit(msg.data[i+3]) {
					b1 := cast.HexDigitToInt(msg.data[i+2]) * 16
					b2 := cast.HexDigitToInt(msg.data[i+3])
					b := byte(b1 + b2)
					buf.WriteByte(b)
					i += 3
				} else if c == '\\' && i < len(msg.data)-1 {
					i++
					switch c = msg.data[i]; c {
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
				if c == '\\' && i < len(msg.data)-1 && msg.data[i+1] == '\'' {
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
				if i == len(msg.data)-1 {
					done = true
				}
			}
		}
		ret = append(ret, buf.String())
	}
}
