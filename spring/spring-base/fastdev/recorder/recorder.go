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
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/util"
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
	util.MustTestMode()
	recorder.mode = mode
}

type recordSession struct {
	session *fastdev.Session
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
func StartRecord(ctx context.Context, sessionID string) (context.Context, error) {
	r := &recordSession{session: &fastdev.Session{
		Session:   sessionID,
		Timestamp: chrono.Now(ctx).UnixNano(),
	}}
	_, loaded := recorder.data.LoadOrStore(sessionID, r)
	if loaded {
		return nil, errors.New("session already started")
	}
	if err := knife.Store(ctx, sessionIDKey, sessionID); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, ctxRecordKey, true), nil
}

// StopRecord 停止流量录制
func StopRecord(ctx context.Context) (*fastdev.Session, error) {
	var ret *fastdev.Session
	err := onSession(ctx, func(r *recordSession) error {
		recorder.data.Delete(r.session.Session)
		r.close = true
		ret = r.session
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// RecordInbound 录制 inbound 流量。
func RecordInbound(ctx context.Context, inbound *fastdev.Action) error {
	return onSession(ctx, func(r *recordSession) error {
		if r.close {
			return errors.New("recording already stopped")
		}
		if r.session.Inbound != nil {
			return errors.New("inbound already set")
		}
		inbound.Timestamp = chrono.Now(ctx).UnixNano()
		r.session.Inbound = inbound
		return nil
	})
}

// RecordAction 录制 outbound 流量。
func RecordAction(ctx context.Context, action *fastdev.Action) error {
	return onSession(ctx, func(r *recordSession) error {
		if r.close {
			return errors.New("recording already stopped")
		}
		action.Timestamp = r.session.Timestamp
		r.session.Actions = append(r.session.Actions, action)
		return nil
	})
}

func onSession(ctx context.Context, f func(*recordSession) error) error {
	if !recorder.mode {
		return errors.New("record mode not enabled")
	}
	v, err := knife.Load(ctx, sessionIDKey)
	if err != nil {
		return err
	}
	if v == nil {
		return errors.New("session id not found")
	}
	sessionID := v.(string)
	v, ok := recorder.data.Load(sessionID)
	if !ok {
		return errors.New("session not found")
	}
	r := v.(*recordSession)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return f(r)
}
