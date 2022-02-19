// Copyright 2012-2019 the original author or authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package json

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

func MarshalValue(v reflect.Value) ([]byte, error) {
	e := newEncodeState()
	err := e.marshalValue(v, encOpts{escapeHTML: true})
	if err != nil {
		return nil, err
	}
	buf := append([]byte(nil), e.Bytes()...)
	encodeStatePool.Put(e)
	return buf, nil
}

func (e *encodeState) marshalValue(v reflect.Value, opts encOpts) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if je, ok := r.(jsonError); ok {
				err = je.error
			} else {
				panic(r)
			}
		}
	}()
	e.reflectValue(v, opts)
	return nil
}

// stringEncoderV2 对于 "\u0000\xC0\n\t\u0000\xBEm\u0006\x89Z(\u0000\n"
// 这样的字符串，标准库的序列化结果不正确，不能逆向转换回去。新方法是对原字符串多做
// 一次 quote 操作，同时使用 @ 前缀指示是否进行二次 quote 以便能够正确反序列化。
func stringEncoderV2(e *encodeState, v reflect.Value, opts encOpts) {
	if v.Type() == numberType {
		numStr := v.String()
		// In Go1.5 the empty string encodes to "0", while this is not a valid number literal
		// we keep compatibility so check validity after this.
		if numStr == "" {
			numStr = "0" // Number's zero-val
		}
		if !isValidNumber(numStr) {
			e.error(fmt.Errorf("json: invalid number literal %q", numStr))
		}
		if opts.quoted {
			e.WriteByte('"')
		}
		e.WriteString(numStr)
		if opts.quoted {
			e.WriteByte('"')
		}
		return
	}
	if opts.quoted {
		e2 := newEncodeState()
		// Since we encode the string twice, we only need to escape HTML
		// the first time.
		e2.string(v.String(), opts.escapeHTML)
		e.stringBytes(e2.Bytes(), false)
		encodeStatePool.Put(e2)
	} else {
		e.string(Quote(v.String()), opts.escapeHTML)
	}
}

// NeedQuote 是否需要 quote 操作。
func NeedQuote(s string) bool {
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			i++
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			return true
		}
		i += size
	}
	return false
}

// Quote 添加 (@Quote@) 前缀，反序列化时需要移除。
func Quote(s string) string {
	if NeedQuote(s) {
		return "@" + strconv.Quote(s)
	}
	return s
}

// Unquote 存在 (@Quote@) 前缀，反序列化时需要移除。
func Unquote(s string) (string, error) {
	if strings.HasPrefix(s, "@\"") {
		return strconv.Unquote(s[1:])
	}
	return s, nil
}
