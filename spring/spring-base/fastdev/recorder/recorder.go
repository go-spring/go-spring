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
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/knife"
)

func init() {
	if cast.ToBool(os.Getenv("GS_FASTDEV_RECORD")) {
		recorder.mode = true
	}
}

const (
	sessionIDKey = "::RECORD-SESSION-ID::"
)

var recorder struct {
	mode bool     // 是否为录制模式。
	data sync.Map // 正在录制的数据。
}

// RecordMode 返回是否是录制模式。
func RecordMode() bool {
	return recorder.mode
}

// SetRecordMode 打开或者关闭录制模式，仅用于单元测试。
func SetRecordMode(mode bool) {
	fastdev.CheckTestMode()
	recorder.mode = mode
}

type Message struct {
	data interface{}
}

func NewMessage(data interface{}) *Message {
	return &Message{data: data}
}

func (msg *Message) MarshalJSON() ([]byte, error) {
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

// Session 一次上游调用称为一个会话。
type Session struct {
	Session string    `json:"session,omitempty"` // 会话 ID
	Inbound *Action   `json:"inbound,omitempty"` // 上游数据
	Actions []*Action `json:"actions,omitempty"` // 动作数据
}

// Action 将上下游调用、缓存获取、文件写入等抽象为一个动作。
type Action struct {
	Protocol  string   `json:"protocol,omitempty"` // 协议名称
	Request   *Message `json:"request,omitempty"`  // 请求内容
	Response  *Message `json:"response,omitempty"` // 响应内容
	Timestamp int64    `json:"timestamp"`          // 时间戳
}

func ToJson(session *Session) (string, error) {
	b, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ToPrettyJson(session *Session) (string, error) {
	b, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type recordSession struct {
	session *Session
	mutex   sync.Mutex
	close   bool
}

func onSession(ctx context.Context, f func(*recordSession) error) (*recordSession, error) {
	if !recorder.mode {
		return nil, errors.New("record mode not enabled")
	}
	v, ok := knife.Get(ctx, sessionIDKey)
	if !ok {
		return nil, errors.New("session id not found")
	}
	sessionID := v.(string)
	v, ok = recorder.data.Load(sessionID)
	if !ok {
		return nil, errors.New("session not found")
	}
	r := v.(*recordSession)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if err := f(r); err != nil {
		return nil, err
	}
	return r, nil
}

// StartRecord 开始流量录制
func StartRecord(ctx context.Context, sessionID string) error {
	if err := knife.Set(ctx, sessionIDKey, sessionID); err != nil {
		return err
	}
	r := &recordSession{session: &Session{Session: sessionID}}
	_, loaded := recorder.data.LoadOrStore(sessionID, r)
	if loaded {
		return errors.New("session already started")
	}
	return nil
}

// StopRecord 停止流量录制
func StopRecord(ctx context.Context) (*Session, error) {
	r, err := onSession(ctx, func(r *recordSession) error {
		r.close = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.session, nil
}

// RecordInbound 录制 inbound 流量。
func RecordInbound(ctx context.Context, inbound *Action) error {
	_, err := onSession(ctx, func(r *recordSession) error {
		if r.close {
			return errors.New("recording already stopped")
		}
		if r.session.Inbound != nil {
			return errors.New("inbound already set")
		}
		r.session.Inbound = inbound
		return nil
	})
	return err
}

// RecordAction 录制 outbound 流量。
func RecordAction(ctx context.Context, action *Action) error {
	_, err := onSession(ctx, func(r *recordSession) error {
		if r.close {
			return errors.New("recording already stopped")
		}
		r.session.Actions = append(r.session.Actions, action)
		return nil
	})
	return err
}
