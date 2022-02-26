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
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
)

const (
	loggerTag  = "_recorder_tag"
	sessionKey = "::RECORD-SESSION-ID::"
)

func init() {
	if cast.ToBool(os.Getenv("GS_FASTDEV_RECORD")) {
		recorder.mode = true
		SetLogger(log.Console)
	}
}

// SetLogger 设置录制模块使用的日志。
func SetLogger(o log.Logger) {
	log.RegisterLogger(o, loggerTag)
}

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
	util.MustTestMode()
	recorder.mode = mode
}

type recordSession struct {
	session *Session
	mutex   sync.Mutex
	close   bool
}

type ctxRecordKeyType int

var ctxRecordKey ctxRecordKeyType

// EnableRecord 从 context.Context 对象是否开启流量录制。
func EnableRecord(ctx context.Context) bool {
	return ctx.Value(ctxRecordKey) == true
}

// StartRecord 开始流量录制
func StartRecord(ctx context.Context, sessionID string) context.Context {
	var err error
	defer func() {
		if err != nil {
			log.WithSkip(1).Error(err)
		}
	}()
	r := &recordSession{session: &Session{
		Session:   sessionID,
		Timestamp: clock.Now(ctx).UnixNano(),
	}}
	_, loaded := recorder.data.LoadOrStore(sessionID, r)
	if loaded {
		err = errors.New("session already started")
		return ctx
	}
	err = knife.Store(ctx, sessionKey, sessionID)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, ctxRecordKey, true)
}

// StopRecord 停止流量录制
func StopRecord(ctx context.Context) *Session {
	var ret *Session
	onSession(ctx, func(r *recordSession) error {
		recorder.data.Delete(r.session.Session)
		r.close = true
		ret = r.session
		return nil
	})
	return ret
}

type SimpleAction struct {
	Protocol  string        `json:",omitempty"` // 协议名称
	Timestamp int64         `json:",omitempty"` // 时间戳
	Request   func() string `json:",omitempty"` // 请求内容
	Response  func() string `json:",omitempty"` // 响应内容
}

func toAction(action *SimpleAction) *Action {
	return &Action{
		Protocol:  action.Protocol,
		Timestamp: action.Timestamp,
		Request:   Message(action.Request),
		Response:  Message(action.Response),
	}
}

// RecordInbound 录制 inbound 流量。
func RecordInbound(ctx context.Context, protocol string, inbound *SimpleAction) {
	onSession(ctx, func(r *recordSession) error {
		if r.close {
			return errors.New("recording already stopped")
		}
		if r.session.Inbound != nil {
			return errors.New("inbound already set")
		}
		inbound.Timestamp = clock.Now(ctx).UnixNano()
		inbound.Protocol = protocol
		r.session.Inbound = toAction(inbound)
		return nil
	})
}

// RecordAction 录制 outbound 流量。
func RecordAction(ctx context.Context, protocol string, action *SimpleAction) {
	onSession(ctx, func(r *recordSession) error {
		if r.close {
			return errors.New("recording already stopped")
		}
		action.Timestamp = clock.Now(ctx).UnixNano()
		action.Protocol = protocol
		r.session.Actions = append(r.session.Actions, toAction(action))
		return nil
	})
}

func onSession(ctx context.Context, f func(*recordSession) error) {
	var err error
	defer func() {
		if err != nil {
			log.WithSkip(2).Error(err)
		}
	}()
	if !recorder.mode {
		err = errors.New("record mode not enabled")
		return
	}
	v, err := knife.Load(ctx, sessionKey)
	if err != nil {
		return
	}
	if v == nil {
		err = errors.New("session id not found")
		return
	}
	sessionID := v.(string)
	v, ok := recorder.data.Load(sessionID)
	if !ok {
		err = errors.New("session not found")
		return
	}
	r := v.(*recordSession)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	err = f(r)
}
