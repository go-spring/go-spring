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
	"reflect"
	"strconv"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/net/internal/json"
)

type Message func() string

func (f Message) Data() string {
	return f()
}

func (f Message) MarshalJSON() ([]byte, error) {
	return json.Marshal(f())
}

type Session struct {
	Session   string    `json:",omitempty"` // 会话 ID
	Timestamp int64     `json:",omitempty"` // 时间戳
	Inbound   *Action   `json:",omitempty"` // 上游数据
	Actions   []*Action `json:",omitempty"` // 动作数据
}

type Action struct {
	Protocol  string  `json:",omitempty"` // 协议名称
	Timestamp int64   `json:",omitempty"` // 时间戳
	Request   Message `json:",omitempty"` // 请求内容
	Response  Message `json:",omitempty"` // 响应内容
}

type RawSession struct {
	Session   string       `json:",omitempty"` // 会话 ID
	Timestamp int64        `json:",omitempty"` // 时间戳
	Inbound   *RawAction   `json:",omitempty"` // 上游数据
	Actions   []*RawAction `json:",omitempty"` // 动作数据
}

type RawAction struct {
	Protocol  string `json:",omitempty"` // 协议名称
	Timestamp int64  `json:",omitempty"` // 时间戳
	Request   string `json:",omitempty"` // 请求内容
	Response  string `json:",omitempty"` // 响应内容
}

func ToJson(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func ToJsonE(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func ToPrettyJson(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func ToPrettyJsonE(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

func ToJsonValue(v reflect.Value) string {
	b, err := json.MarshalValue(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func ToRawSession(data string) (*RawSession, error) {
	var session *RawSession
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, err
	}
	return session, nil
}

const rootKey = "$"

var (
	numberType = reflect.TypeOf(json.Number(""))
)

func FlatJSON(b []byte) map[string]string {
	result := make(map[string]string)
	if !flatJSON(rootKey, b, result) {
		result[rootKey] = string(b)
	}
	return result
}

func flatJSON(prefix string, b []byte, result map[string]string) bool {
	var v interface{}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return false
	}
	flatValue(prefix, v, result)
	return true
}

func flatValue(prefix string, v interface{}, result map[string]string) {
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		result[prefix] = "null"
		return
	}
	switch val.Kind() {
	case reflect.Map:
		if val.Len() == 0 {
			result[prefix] = "{}"
			return
		}
		iter := val.MapRange()
		for iter.Next() {
			key := cast.ToString(iter.Key().Interface())
			key = prefix + "[" + key + "]"
			flatValue(key, iter.Value().Interface(), result)
		}
	case reflect.Array, reflect.Slice:
		if val.Len() == 0 {
			result[prefix] = "[]"
			return
		}
		for i := 0; i < val.Len(); i++ {
			key := prefix + "[" + strconv.Itoa(i) + "]"
			flatValue(key, val.Index(i).Interface(), result)
		}
	case reflect.Bool:
		result[prefix] = strconv.FormatBool(val.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result[prefix] = strconv.FormatInt(val.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result[prefix] = strconv.FormatUint(val.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		result[prefix] = strconv.FormatFloat(val.Float(), 'f', -1, 64)
	case reflect.String:
		if val.Type() == numberType {
			result[prefix] = val.String()
		} else {
			flatString(prefix, val.String(), result)
		}
	}
}

func flatString(prefix string, str string, result map[string]string) {
	trimBytes := bytes.TrimSpace([]byte(str))
	if len(trimBytes) == 0 {
		result[prefix] = strconv.Quote(str)
		return
	}
	switch trimBytes[0] {
	case '{', '[', '"':
		if !flatJSON(prefix+`[""]`, trimBytes, result) {
			result[prefix] = strconv.Quote(str)
		}
	default:
		result[prefix] = strconv.Quote(str)
	}
}
