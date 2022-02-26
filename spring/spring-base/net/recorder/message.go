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
	"reflect"

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
