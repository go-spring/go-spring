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

package fastdev

import (
	"context"
	"errors"
	"sync"

	"github.com/go-spring/spring-base/knife"
)

type replayData struct {
	session *Session
	matches sync.Map
}

var replayer struct {
	mode bool     // 是否是回放模式。
	data sync.Map // 正在回放的数据。
}

// ReplayMode 返回是否是回放模式。
func ReplayMode() bool {
	return replayer.mode
}

// Delete 删除 sessionID 对应的回放数据。
func Delete(sessionID string) {
	replayer.data.Delete(sessionID)
}

// Store 存储 sessionID 对应的回放数据。
func Store(session *Session) {
	replayer.data.Store(session.Session, &replayData{session: session})
}

// ReplayAction 根据 action 传入的匹配信息返回对应的响应数据。
func ReplayAction(ctx context.Context, action *Action) (bool, error) {

	if !replayer.mode {
		return false, errors.New("replay mode not enabled")
	}

	sessionID, ok := knife.Get(ctx, ReplaySessionIDKey)
	if !ok {
		return false, errors.New("session id not found")
	}

	value, ok := replayer.data.Load(sessionID.(string))
	if !ok {
		return false, errors.New("session not found")
	}

	data := value.(*replayData)
	actions := data.session.Actions

	for i, r := range actions {
		if r.Protocol != action.Protocol {
			continue
		}
		if r.Request != action.Request {
			continue
		}
		if _, loaded := data.matches.LoadOrStore(i, true); loaded {
			continue
		}
		action.Response = r.Response
		return true, nil
	}
	return false, nil
}
