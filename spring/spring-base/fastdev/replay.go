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
	"os"
	"strings"
	"sync"

	"github.com/go-spring/spring-base/knife"
)

var (
	replayMode bool     // 是否是回放模式。
	replayData sync.Map // 正在回放的数据。
)

func init() {
	s := os.Getenv("fastdev_mode")
	ss := strings.Split(s, ",")
	for _, c := range ss {
		if c == "replay" {
			replayMode = true
			runAgent()
			break
		}
	}
}

// ReplayMode 返回是否是回放模式。
func ReplayMode() bool {
	return replayMode
}

// Delete 删除 sessionID 对应的回放数据。
func Delete(sessionID string) {
	replayData.Delete(sessionID)
}

// Store 存储 sessionID 对应的回放数据。
func Store(session *Session) {
	replayData.Store(session.Session, session)
}

// ReplayAction 根据 action 传入的匹配信息返回对应的响应数据。
func ReplayAction(ctx context.Context, action *Action) (bool, error) {

	if !replayMode {
		return false, errors.New("replay mode not enabled")
	}

	sessionID, ok := knife.Get(ctx, ReplaySessionIDKey)
	if !ok {
		return false, errors.New("session id not found")
	}

	session, ok := replayData.Load(sessionID.(string))
	if !ok {
		return false, errors.New("session not found")
	}

	for _, r := range session.(*Session).Actions {
		if r.Protocol != action.Protocol {
			continue
		}
		if r.Request != action.Request {
			continue
		}
		action.Response = r.Response
		return true, nil
	}
	return false, nil
}
