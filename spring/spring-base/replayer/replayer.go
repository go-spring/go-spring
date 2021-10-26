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

// Package replayer 流量回放。
package replayer

import (
	"context"
	"errors"
	"reflect"

	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/recorder"
)

var replayMode bool

// ReplayMode 返回是否是回放模式。
func ReplayMode() bool {
	return replayMode
}

// SetReplayMode 设置为回放模式。
func SetReplayMode() {
	replayMode = true
}

var replayData = map[string]*recorder.Session{}

// Delete 删除 sessionID 对应的回放数据。
func Delete(sessionID string) {
	delete(replayData, sessionID)
}

// Store 存储 sessionID 对应的回放数据。
func Store(sessionID string, session *recorder.Session) {
	replayData[sessionID] = session
}

// SessionIDKey 回放数据 ID 存储使用的 Key 。
const SessionIDKey = "REPLAYER-SESSION-ID"

// Replay 根据 action 传入的匹配信息返回对应的数据。
func Replay(ctx context.Context, action *recorder.Action) (ok bool, err error) {
	sessionID, ok := knife.Get(ctx, SessionIDKey)
	if !ok {
		return false, errors.New("session id not found")
	}
	session, ok := replayData[sessionID.(string)]
	if !ok {
		return false, errors.New("session not found")
	}
	for _, r := range session.Actions {
		if r.Protocol != action.Protocol {
			continue
		}
		if r.Key != action.Key {
			continue
		}
		switch action.Protocol {
		case recorder.REDIS:
			a1 := r.Data.(*recorder.Redis)
			a2 := action.Data.(*recorder.Redis)
			if reflect.DeepEqual(a1.Request, a2.Request) {
				a2.Response = a1.Response
				return true, nil
			}
		}
	}
	return false, nil
}
