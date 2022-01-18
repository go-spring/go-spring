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
	"errors"
	"os"
	"sync"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/internal/json"
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

// Session 一次上游调用称为一个会话。
type Session struct {
	Session string    `json:"session,omitempty"` // 会话 ID
	Inbound *Action   `json:"inbound,omitempty"` // 上游数据
	Actions []*Action `json:"actions,omitempty"` // 动作数据
}

// Action 将上下游调用、缓存获取、文件写入等抽象为一个动作。
type Action struct {
	ID        int32       `json:"id"`                 // 序号
	Protocol  string      `json:"protocol,omitempty"` // 协议名称
	Label     string      `json:"label,omitempty"`    // 分类标签
	Request   interface{} `json:"request,omitempty"`  // 请求内容
	Response  interface{} `json:"response,omitempty"` // 响应内容
	Timestamp int64       `json:"timestamp"`          // 时间戳
}

func ToJson(session *Session) (string, error) {
	b, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ToPrettyJson(session *Session, indent string) (string, error) {
	b, err := json.MarshalIndent(session, "", indent)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type recordSession struct {
	session *Session
	mutex   sync.Mutex
	count   int32
}

func (r *recordSession) lock(f func()) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	f()
}

func getSessionID(ctx context.Context) (string, error) {
	if !recorder.mode {
		return "", errors.New("record mode not enabled")
	}
	v, ok := knife.Get(ctx, sessionIDKey)
	if !ok {
		return "", errors.New("no session id found")
	}
	sessionID, ok := v.(string)
	if !ok {
		return "", errors.New("session id isn't string")
	}
	return sessionID, nil
}

func setSessionID(ctx context.Context, sessionID string) error {
	return knife.Set(ctx, sessionIDKey, sessionID)
}

func getRecordSession(ctx context.Context) (*recordSession, error) {
	sessionID, err := getSessionID(ctx)
	if err != nil {
		return nil, err
	}
	v, ok := recorder.data.Load(sessionID)
	if ok {
		return v.(*recordSession), nil
	}
	return nil, errors.New("recording isn't started")
}

func saveRecordSession(r *recordSession) error {
	_, loaded := recorder.data.LoadOrStore(r.session.Session, r)
	if loaded {
		return errors.New("session already started")
	}
	return nil
}

// StartRecord 开始流量录制
func StartRecord(ctx context.Context) (string, error) {
	sessionID := fastdev.NewSessionID()
	err := setSessionID(ctx, sessionID)
	if err != nil {
		return "", err
	}
	err = saveRecordSession(&recordSession{
		session: &Session{Session: sessionID},
	})
	if err != nil {
		return "", err
	}
	return sessionID, nil
}

// StopRecord 停止流量录制
func StopRecord(ctx context.Context) (*Session, error) {
	r, err := getRecordSession(ctx)
	if err != nil {
		return nil, err
	}
	recorder.data.Delete(r.session.Session)
	return r.session, nil
}

// RecordInbound 录制 inbound 流量。
func RecordInbound(ctx context.Context, inbound *Action) error {
	r, err := getRecordSession(ctx)
	if err != nil {
		return err
	}
	r.lock(func() {
		inbound.ID = r.count
		r.count++
		r.session.Inbound = inbound
	})
	return nil
}

// RecordAction 录制 outbound 流量。
func RecordAction(ctx context.Context, action *Action) error {
	r, err := getRecordSession(ctx)
	if err != nil {
		return err
	}
	r.lock(func() {
		action.ID = r.count
		r.count++
		r.session.Actions = append(r.session.Actions, action)
	})
	return nil
}
