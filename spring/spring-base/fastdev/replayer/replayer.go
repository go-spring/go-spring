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
	"github.com/go-spring/spring-base/chrono"
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

type Session struct {
	Session   string    `json:",omitempty"` // 会话 ID
	Timestamp int64     `json:",omitempty"` // 时间戳
	Inbound   *Action   `json:",omitempty"` // 上游数据
	Actions   []*Action `json:",omitempty"` // 动作数据
}

func (session *Session) Flat() error {
	if err := session.Inbound.Flat(); err != nil {
		return err
	}
	for _, action := range session.Actions {
		if err := action.Flat(); err != nil {
			return err
		}
	}
	return nil
}

func (session *Session) String() (string, error) {
	b, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (session *Session) Pretty() (string, error) {
	b, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type Action struct {
	Protocol        string            `json:",omitempty"` // 协议名称
	Timestamp       int64             `json:",omitempty"` // 时间戳
	Request         string            `json:",omitempty"` // 请求内容
	Response        string            `json:",omitempty"` // 响应内容
	FlatRequest     map[string]string `json:",omitempty"` // 请求内容
	FlatResponse    map[string]string `json:",omitempty"` // 响应内容
	RecTimestamp    int64             `json:",omitempty"` // 时间戳
	RecRequest      string            `json:",omitempty"` // 请求内容
	RecResponse     string            `json:",omitempty"` // 响应内容
	RecFlatRequest  map[string]string `json:",omitempty"` // 请求内容
	RecFlatResponse map[string]string `json:",omitempty"` // 响应内容
}

func (action *Action) Flat() error {
	p := fastdev.GetProtocol(action.Protocol)
	if p == nil {
		return errors.New("invalid protocol")
	}
	var err error
	action.FlatRequest, err = p.FlatRequest(action.Request)
	if err != nil {
		return err
	}
	action.FlatResponse, err = p.FlatResponse(action.Response)
	if err != nil {
		return err
	}
	action.RecFlatRequest, err = p.FlatRequest(action.RecRequest)
	if err != nil {
		return err
	}
	action.RecFlatResponse, err = p.FlatResponse(action.RecResponse)
	if err != nil {
		return err
	}
	return nil
}

func (action *Action) String() (string, error) {
	b, err := json.Marshal(action)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (action *Action) Pretty() (string, error) {
	b, err := json.MarshalIndent(action, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func ToSession(session *fastdev.RawSession) (*Session, error) {
	var actions []*Action
	for _, action := range session.Actions {
		p := fastdev.GetProtocol(action.Protocol)
		if p == nil {
			return nil, errors.New("invalid protocol")
		}
		if p.ShouldDiff() {
			actions = append(actions, ToAction(action))
		}
	}
	return &Session{
		Session:   session.Session,
		Timestamp: session.Timestamp,
		Inbound:   ToAction(session.Inbound),
		Actions:   actions,
	}, nil
}

func ToAction(action *fastdev.RawAction) *Action {
	return &Action{
		Protocol:  action.Protocol,
		Timestamp: action.Timestamp,
		Request:   action.Request,
		Response:  action.Response,
	}
}

type replayData struct {
	session *Session
	matched sync.Map
	actions map[string]map[string][]*Action
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
		p := fastdev.GetProtocol(a.Protocol)
		if p == nil {
			return errors.New("invalid protocol")
		}
		m, ok := actions[a.Protocol]
		if !ok {
			m = make(map[string][]*Action)
			actions[a.Protocol] = m
		}
		label := p.GetLabel(a.Request)
		m[label] = append(m[label], a)
	}
	r.actions = actions
	return nil
}

// Delete 删除 sessionID 对应的回放数据。
func Delete(sessionID string) {
	replayer.data.Delete(sessionID)
}

func GetSessionID(ctx context.Context) (string, error) {
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

func getReplayData(ctx context.Context) (*replayData, error) {
	sessionID, err := GetSessionID(ctx)
	if err != nil {
		return nil, err
	}
	v, ok := replayer.data.Load(sessionID)
	if !ok {
		return nil, errors.New("session not found")
	}
	return v.(*replayData), nil
}

func ReplayInbound(ctx context.Context, response string) error {
	r, err := getReplayData(ctx)
	if err != nil {
		return err
	}
	r.session.Inbound.RecResponse = response
	r.session.Inbound.RecRequest = r.session.Inbound.Request
	r.session.Inbound.RecTimestamp = chrono.Now(ctx).UnixNano()
	return nil
}

func ReplayAction(ctx context.Context, protocol string, request string) (*Action, error) {

	r, err := getReplayData(ctx)
	if err != nil {
		return nil, err
	}

	p := fastdev.GetProtocol(protocol)
	if p == nil {
		return nil, errors.New("invalid protocol")
	}

	m, ok := r.actions[protocol]
	if !ok {
		return nil, errors.New("invalid protocol")
	}

	label := p.GetLabel(request)
	for _, action := range m[label] {
		if action.Request != request { // TODO 改为模糊匹配方式
			continue
		}
		if _, loaded := r.matched.LoadOrStore(action, true); loaded {
			continue
		}
		action.RecRequest = request
		action.RecResponse = action.Response
		action.RecTimestamp = chrono.Now(ctx).UnixNano()
		return action, nil
	}
	return nil, nil
}
