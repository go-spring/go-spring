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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/recorder"
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

type Message struct {
	data string
}

func (msg *Message) ToValue(i interface{}) error {
	if strings.HasPrefix(msg.data, "@\"") {
		s, err := strconv.Unquote(msg.data[1:])
		if err != nil {
			return err
		}
		v := reflect.ValueOf(i).Elem()
		switch i.(type) {
		case *string:
			v.Set(reflect.ValueOf(s))
			return nil
		case *[]byte:
			v.Set(reflect.ValueOf([]byte(s)))
			return nil
		default:
			return fmt.Errorf("expect *string or *[]byte but %T", i)
		}
	}
	return json.Unmarshal([]byte(msg.data), i)
}

func (msg *Message) UnmarshalJSON(data []byte) error {
	if data[0] != '"' {
		msg.data = string(data)
		return nil
	}
	s, err := strconv.Unquote(string(data))
	if err != nil {
		return err
	}
	msg.data = s
	return nil
}

type Session struct {
	Session string    `json:"session,omitempty"` // 会话 ID
	Inbound *Action   `json:"inbound,omitempty"` // 上游数据
	Actions []*Action `json:"actions,omitempty"` // 动作数据
}

type Action struct {
	Protocol  string   `json:"protocol,omitempty"` // 协议名称
	Label     string   `json:"-"`                  // 分类标签
	Request   *Message `json:"request,omitempty"`  // 请求内容
	Response  *Message `json:"response,omitempty"` // 响应内容
	Timestamp int64    `json:"timestamp"`          // 时间戳
}

func ToSession(data string) (*Session, error) {
	var s *Session
	err := json.Unmarshal([]byte(data), &s)
	if err != nil {
		return nil, err
	}
	return s, nil
}

type replayData struct {
	session *Session
	actions map[string]map[string][]*Action
	matched sync.Map
}

// Store 存储 sessionID 对应的回放数据。
func Store(session *Session) error {

	r := &replayData{session: session}
	_, loaded := replayer.data.LoadOrStore(session.Session, r)
	if loaded {
		return errors.New("session already stored")
	}

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
		label := fastdev.GetLabelStrategy(a.Protocol).GetLabel(a.Request.data)
		p[label] = append(p[label], a)
	}
	r.actions = actions
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
func GetAction(ctx context.Context, action *recorder.Action) (*Action, error) {

	sessionID, err := getSessionID(ctx)
	if err != nil {
		return nil, err
	}

	v, ok := replayer.data.Load(sessionID)
	if !ok {
		return nil, errors.New("session not found")
	}

	p := v.(*replayData)
	m, ok := p.actions[action.Protocol]
	if !ok {
		return nil, errors.New("invalid protocol")
	}

	data, err := json.Marshal(action.Request)
	if err != nil {
		return nil, err
	}

	var req *Message
	err = json.Unmarshal(data, &req)
	if err != nil {
		return nil, err
	}

	label := fastdev.GetLabelStrategy(action.Protocol).GetLabel(req.data)
	for _, r := range m[label] {
		if r.Request.data != req.data {
			continue
		}
		if _, loaded := p.matched.LoadOrStore(action, true); loaded {
			continue
		}
		return r, nil
	}
	return nil, nil
}
