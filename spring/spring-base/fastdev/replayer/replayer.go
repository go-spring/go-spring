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

package replayer

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
	if cast.ToBool(os.Getenv("GS_FASTDEV_REPLAY")) {
		replayer.mode = true
	}
}

const (
	sessionIDKey = "REPLAY-SESSION-ID"
)

var replayer struct {
	mode bool     // 是否是回放模式。
	data sync.Map // 正在回放的数据。
}

// ReplayMode 返回是否是回放模式。
func ReplayMode() bool {
	return replayer.mode
}

// SetReplayMode 打开或者关闭回放模式，仅用于单元测试。
func SetReplayMode(mode bool) {
	fastdev.CheckTestMode()
	replayer.mode = mode
}

type RawMessage []byte

func (raw RawMessage) ToValue(v interface{}) error {
	return json.Unmarshal(raw, v)
}

func (raw *RawMessage) UnmarshalJSON(data []byte) error {
	*raw = append((*raw)[0:0], data...)
	return nil
}

type Session struct {
	Session string    `json:"session,omitempty"` // 会话 ID
	Inbound *Action   `json:"inbound,omitempty"` // 上游数据
	Actions []*Action `json:"actions,omitempty"` // 动作数据
}

type Action struct {
	ID        int32      `json:"id"`                 // 序号
	Protocol  string     `json:"protocol,omitempty"` // 协议名称
	Label     string     `json:"label,omitempty"`    // 分类标签
	Request   string     `json:"request,omitempty"`  // 请求内容
	Response  RawMessage `json:"response,omitempty"` // 响应内容
	Timestamp int64      `json:"timestamp"`          // 时间戳
}

// ToSession 反序列化 *Session 对象。
func ToSession(data string) (*Session, error) {
	var session *Session
	err := json.Unmarshal([]byte(data), &session)
	if err != nil {
		return nil, err
	}
	return session, nil
}

type replayData struct {
	session *Session
	actions map[string]map[string][]*Action
	matched sync.Map
}

// Store 存储 sessionID 对应的回放数据。
func Store(session *Session) error {

	actions := make(map[string]map[string][]*Action)
	for _, a := range session.Actions {
		var (
			ok bool
			p  map[string][]*Action
		)
		if p, ok = actions[a.Protocol]; !ok {
			p = make(map[string][]*Action)
			actions[a.Protocol] = p
		}
		p[a.Label] = append(p[a.Label], a)
	}

	_, loaded := replayer.data.LoadOrStore(session.Session, &replayData{
		session: session,
		actions: actions,
	})
	if loaded {
		return errors.New("session already stored")
	}
	return nil
}

// Delete 删除 sessionID 对应的回放数据。
func Delete(sessionID string) {
	replayer.data.Delete(sessionID)
}

func getSessionID(ctx context.Context) (string, error) {
	if !replayer.mode {
		return "", errors.New("replay mode not enabled")
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

func SetSessionID(ctx context.Context, sessionID string) error {
	return knife.Set(ctx, sessionIDKey, sessionID)
}

// GetAction 根据 action 传入的匹配信息返回对应的响应数据。
func GetAction(ctx context.Context, action *Action) (bool, error) {

	sessionID, err := getSessionID(ctx)
	if err != nil {
		return false, err
	}

	v, ok := replayer.data.Load(sessionID)
	if !ok {
		return false, errors.New("session not found")
	}

	p := v.(*replayData)
	m, ok := p.actions[action.Protocol]
	if !ok {
		return false, errors.New("invalid protocol")
	}

	actions, ok := m[action.Label]
	if !ok {
		return false, errors.New("invalid label")
	}

	for _, r := range actions {
		if r.Request != action.Request {
			continue
		}
		if _, loaded := p.matched.LoadOrStore(r.ID, true); loaded {
			continue
		}
		action.Response = r.Response
		return true, nil
	}
	return false, nil
}
