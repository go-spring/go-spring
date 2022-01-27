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
	"encoding/json"
	"reflect"
	"strconv"
	"unicode/utf8"

	"github.com/go-spring/spring-base/cast"
)

type Message interface {
	json.Marshaler
}

type message struct {
	data interface{}
}

func NewMessage(data interface{}) Message {
	return &message{data: data}
}

func (msg *message) MarshalJSON() ([]byte, error) {
	v := reflect.ValueOf(msg.data)
	v = reflect.Indirect(v)
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s := strconv.FormatInt(v.Int(), 10)
		return []byte(s), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s := strconv.FormatUint(v.Uint(), 10)
		return []byte(s), nil
	case reflect.Float32, reflect.Float64:
		s := strconv.FormatFloat(v.Float(), 'f', -1, 64)
		return []byte(s), nil
	case reflect.Bool:
		s := strconv.FormatBool(v.Bool())
		return []byte(s), nil
	case reflect.String:
		s := v.String()
		if c := quoteCount(s); c == 0 {
			return []byte("\"" + s + "\""), nil
		} else if c == 1 {
			return []byte(strconv.Quote(s)), nil
		} else {
			return []byte(strconv.Quote("@" + strconv.Quote(s))), nil
		}
	default:
		return json.Marshal(msg.data)
	}
}

type cmdLine struct {
	data []interface{}
}

func NewCommandLine(data ...interface{}) Message {
	return &cmdLine{data: data}
}

func (msg *cmdLine) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	for i, arg := range msg.data {
		switch s := arg.(type) {
		case string:
			if c := quoteCount(s); c > 1 {
				s = strconv.Quote(s)
			}
			buf.WriteString(s)
		default:
			buf.WriteString(cast.ToString(arg))
		}
		if i < len(msg.data)-1 {
			buf.WriteByte(' ')
		}
	}
	return NewMessage(buf.String()).MarshalJSON()
}

type csv struct {
	data []interface{}
}

func NewCSV(data ...interface{}) Message {
	return &csv{data: data}
}

func (msg *csv) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	for i, arg := range msg.data {
		switch s := arg.(type) {
		case string:
			if c := quoteCount(s); c == 1 {
				s = strconv.Quote(s)
			}
			buf.WriteString(strconv.Quote(s))
		default:
			buf.WriteString(strconv.Quote(cast.ToString(arg)))
		}
		if i < len(msg.data)-1 {
			buf.WriteByte(',')
		}
	}
	return NewMessage(buf.String()).MarshalJSON()
}

// quoteCount 查询 quote 的次数。
func quoteCount(s string) int {
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
