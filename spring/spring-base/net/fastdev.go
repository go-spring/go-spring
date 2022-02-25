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

package net

//
//import (
//	"fmt"
//	"reflect"
//
//	"github.com/go-spring/spring-base/fastdev/internal/json"
//)
//
//const (
//	HTTP  = "HTTP"
//	SQL   = "SQL"
//	REDIS = "REDIS"
//	APCU  = "APCU"
//)
//
//var (
//	protocols = map[string]Protocol{}
//)
//
//type Protocol interface {
//	ShouldDiff() bool
//	GetLabel(data string) string
//	FlatRequest(data string) (map[string]string, error)
//	FlatResponse(data string) (map[string]string, error)
//}
//
//func GetProtocol(name string) Protocol {
//	return protocols[name]
//}
//
//func RegisterProtocol(name string, protocol Protocol) {
//	if _, ok := protocols[name]; ok {
//		panic(fmt.Errorf("%s: duplicate registration", name))
//	}
//	protocols[name] = protocol
//}
//
//type Message func() string
//
//func NewMessage(f func() string) Message {
//	return f
//}
//
//func (msg Message) Data() string {
//	return msg()
//}
//
//func (msg Message) MarshalJSON() ([]byte, error) {
//	return json.Marshal(msg())
//}
//
//type Session struct {
//	Session   string    `json:",omitempty"` // 会话 ID
//	Timestamp int64     `json:",omitempty"` // 时间戳
//	Inbound   *Action   `json:",omitempty"` // 上游数据
//	Actions   []*Action `json:",omitempty"` // 动作数据
//}
//
//func (session *Session) Json() (string, error) {
//	b, err := json.Marshal(session)
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//func (session *Session) PrettyJson() (string, error) {
//	b, err := json.MarshalIndent(session, "", "  ")
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//type Action struct {
//	Protocol  string  `json:",omitempty"` // 协议名称
//	Timestamp int64   `json:",omitempty"` // 时间戳
//	Request   Message `json:",omitempty"` // 请求内容
//	Response  Message `json:",omitempty"` // 响应内容
//}
//
//func (action *Action) Json() (string, error) {
//	b, err := json.Marshal(action)
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//func (action *Action) PrettyJson() (string, error) {
//	b, err := json.MarshalIndent(action, "", "  ")
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//type RawSession struct {
//	Session   string       `json:",omitempty"` // 会话 ID
//	Timestamp int64        `json:",omitempty"` // 时间戳
//	Inbound   *RawAction   `json:",omitempty"` // 上游数据
//	Actions   []*RawAction `json:",omitempty"` // 动作数据
//}
//
//func (session *RawSession) Json() (string, error) {
//	b, err := json.Marshal(session)
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//func (session *RawSession) PrettyJson() (string, error) {
//	b, err := json.MarshalIndent(session, "", "  ")
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//type RawAction struct {
//	Protocol  string `json:",omitempty"` // 协议名称
//	Timestamp int64  `json:",omitempty"` // 时间戳
//	Request   string `json:",omitempty"` // 请求内容
//	Response  string `json:",omitempty"` // 响应内容
//}
//
//func (action *RawAction) Json() (string, error) {
//	b, err := json.Marshal(action)
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//func (action *RawAction) PrettyJson() (string, error) {
//	b, err := json.MarshalIndent(action, "", "  ")
//	if err != nil {
//		return "", err
//	}
//	return string(b), nil
//}
//
//func ToRawSession(data string) (*RawSession, error) {
//	var session *RawSession
//	if err := json.Unmarshal([]byte(data), &session); err != nil {
//		return nil, err
//	}
//	return session, nil
//}
//
//func ToJson(v interface{}) string {
//	b, err := json.Marshal(v)
//	if err != nil {
//		return err.Error()
//	}
//	return string(b)
//}
//
//func ToJsonValue(v reflect.Value) string {
//	b, err := json.MarshalValue(v)
//	if err != nil {
//		return err.Error()
//	}
//	return string(b)
//}
//
//func ToPrettyJson(v interface{}) string {
//	b, err := json.MarshalIndent(v, "", "  ")
//	if err != nil {
//		return err.Error()
//	}
//	return string(b)
//}
