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

// Package fastdev 流量录制和回放，名称源自 didi fastdev 项目。
package fastdev

import (
	"bytes"
	"context"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/fastdev/json"
	"github.com/go-spring/spring-base/knife"
	"github.com/google/uuid"
)

// RecordSessionIDKey 流量录制模式下存储会话 ID 使用的 Key 。
const RecordSessionIDKey = "RECORD-SESSION-ID"

// ReplaySessionIDKey 流量回放模式下存储会话 ID 使用的 Key 。
const ReplaySessionIDKey = "REPLAY-SESSION-ID"

// NewSessionID 使用 uuid 算法生成新的 Session ID 。
func NewSessionID() string {
	u := uuid.New()
	buf := make([]byte, 32)
	hex.Encode(buf, u[:4])
	hex.Encode(buf[8:12], u[4:6])
	hex.Encode(buf[12:16], u[6:8])
	hex.Encode(buf[16:20], u[8:10])
	hex.Encode(buf[20:], u[10:])
	return string(buf)
}

// GetReplaySessionID 获取绑定在 context.Context 对象上的 Session ID 。
func GetReplaySessionID(ctx context.Context) string {
	var s string
	_, _ = knife.Fetch(ctx, ReplaySessionIDKey, &s)
	return s
}

const (
	HTTP  = "http"
	REDIS = "redis"
	APCU  = "apcu"
)

// Action 将上下游调用、缓存获取、文件写入等抽象为一个动作。
type Action struct {
	Protocol  string      `json:"protocol,omitempty"` // 协议名称
	Request   interface{} `json:"request,omitempty"`  // 请求内容
	Response  interface{} `json:"response,omitempty"` // 响应内容
	Timestamp int64       `json:"timestamp"`          // 时间戳
}

// Session 一次上游调用称为一个会话。
type Session struct {
	Session string    `json:"session,omitempty"` // 会话 ID
	Inbound *Action   `json:"inbound,omitempty"` // 上游数据
	Actions []*Action `json:"actions,omitempty"` // 动作数据
}

func (s *Session) ToJson() string {
	b, err := json.Marshal(s)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

//////////////////////////// for test ////////////////////////////

type rawAction struct {
	Protocol  string          `json:"protocol,omitempty"` // 协议名称
	Request   json.RawMessage `json:"request,omitempty"`  // 请求内容
	Response  json.RawMessage `json:"response,omitempty"` // 响应内容
	Timestamp int64           `json:"timestamp"`          // 时间戳
}

type rawSession struct {
	Session string       `json:"session,omitempty"` // 会话 ID
	Inbound *rawAction   `json:"inbound,omitempty"` // 上游数据
	Actions []*rawAction `json:"actions,omitempty"` // 动作数据
}

// ToSession 反序列化 *Session 对象，列表项会进行排序。
func ToSession(data []byte, sorted bool) (*Session, error) {

	var s *rawSession
	err := json.Unmarshal(data, &s)
	if err != nil {
		return nil, err
	}

	ret := &Session{Session: s.Session}

	if s.Inbound != nil {
		ret.Inbound, err = toAction(s.Inbound, sorted)
		if err != nil {
			return nil, err
		}
	}

	ret.Actions = make([]*Action, len(s.Actions))
	for i := 0; i < len(s.Actions); i++ {
		ret.Actions[i], err = toAction(s.Actions[i], sorted)
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func toAction(action *rawAction, sorted bool) (ret *Action, err error) {
	ret = &Action{Protocol: action.Protocol}
	ret.Request, err = toVal(action.Request, sorted)
	if err != nil {
		return nil, err
	}
	ret.Response, err = toVal(action.Response, sorted)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

type RawMessageSlice []json.RawMessage

func (p RawMessageSlice) Len() int           { return len(p) }
func (p RawMessageSlice) Less(i, j int) bool { return bytes.Compare(p[i], p[j]) < 0 }
func (p RawMessageSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func toVal(data []byte, sorted bool) (interface{}, error) {

	if data == nil {
		return nil, nil
	}

	if bytes.Equal(data, []byte("(nil)")) {
		return nil, nil
	}

	if bytes.Equal(data, []byte("null")) {
		return nil, nil
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err == nil {
		ret := make(map[string]interface{})
		for k, v := range m {
			var r interface{}
			r, err = toVal(v, sorted)
			if err != nil {
				return nil, err
			}
			ret[k] = r
		}
		return ret, nil
	}

	var a []json.RawMessage
	if err := json.Unmarshal(data, &a); err == nil {
		if sorted {
			sort.Sort(RawMessageSlice(a))
		}
		ret := make([]interface{}, len(a))
		for i, v := range a {
			var r interface{}
			r, err = toVal(v, sorted)
			if err != nil {
				return nil, err
			}
			ret[i] = r
		}
		return ret, nil
	}

	var i interface{}
	if err := json.Unmarshal(data, &i); err != nil {
		return nil, err
	}

	switch s := i.(type) {
	case string:
		if strings.HasPrefix(s, "@\"") {
			b, err := strconv.Unquote(s[1 : len(s)-1])
			if err != nil {
				return nil, err
			}
			return b, nil
		}
	}
	return i, nil
}
