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
	"strings"

	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-replay/internal/json"
	"github.com/go-spring/spring-replay/recorder"
)

func init() {
	switch strings.ToLower(os.Getenv("GS_FASTDEV_REPLAY")) {
	case "remote":
		replayer.enable = true
		replayer.agent = new(RemoteAgent)
	case "local":
		replayer.enable = true
		replayer.agent = new(LocalAgent)
	}
}

const (
	sessionKey = "REPLAY-SESSION-ID"
)

var replayer struct {
	enable bool  // 是否启用回放模式。
	agent  Agent // 本地还是远程回放。
}

// ReplayMode 返回是否是回放模式。
func ReplayMode() bool {
	return replayer.enable
}

// SetReplayMode 打开或者关闭回放模式，仅用于单元测试。
func SetReplayMode(enable bool) {
	util.MustTestMode()
	replayer.enable = enable
}

// SetReplayAgent 设置本地还是远程回放。
func SetReplayAgent(agent Agent) {
	util.MustTestMode()
	replayer.agent = agent
}

type Session struct {
	Session   string    `json:",omitempty"` // 会话 ID
	Timestamp int64     `json:",omitempty"` // 时间戳
	Inbound   *Action   `json:",omitempty"` // 上游数据
	Actions   []*Action `json:",omitempty"` // 动作数据
}

func (session *Session) Flat() error {
	if session.Inbound != nil {
		if err := session.Inbound.Flat(); err != nil {
			return err
		}
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
	FlatRequest     map[string]string `json:",omitempty"`
	FlatResponse    map[string]string `json:",omitempty"`
	RecTimestamp    int64             `json:",omitempty"` // 时间戳
	RecRequest      string            `json:",omitempty"` // 请求内容
	RecResponse     string            `json:",omitempty"` // 响应内容
	RecFlatRequest  map[string]string `json:",omitempty"`
	RecFlatResponse map[string]string `json:",omitempty"`
}

func (action *Action) Flat() error {
	p := recorder.GetProtocol(action.Protocol)
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

func ToAction(action *recorder.RawAction) *Action {
	return &Action{
		Protocol:  action.Protocol,
		Timestamp: action.Timestamp,
		Request:   action.Request,
		Response:  action.Response,
	}
}

func ToSession(session *recorder.RawSession) (*Session, error) {
	var inbound *Action
	if session.Inbound != nil {
		inbound = ToAction(session.Inbound)
	}
	var actions []*Action
	for _, action := range session.Actions {
		p := recorder.GetProtocol(action.Protocol)
		if p == nil {
			return nil, errors.New("invalid protocol")
		}
		actions = append(actions, ToAction(action))
	}
	return &Session{
		Session:   session.Session,
		Timestamp: session.Timestamp,
		Inbound:   inbound,
		Actions:   actions,
	}, nil
}

func GetSessionID(ctx context.Context) (string, error) {
	if !replayer.enable {
		return "", errors.New("replay mode not enabled")
	}
	v, err := knife.Load(ctx, sessionKey)
	if err != nil {
		return "", err
	}
	if v == nil {
		return "", errors.New("no session id found")
	}
	sessionID, ok := v.(string)
	if !ok {
		return "", errors.New("session id isn't string")
	}
	return sessionID, nil
}

func SetSessionID(ctx context.Context, sessionID string) error {
	return knife.Store(ctx, sessionKey, sessionID)
}

func Query(ctx context.Context, protocol, request string) (response string, ok bool, err error) {
	if replayer.agent == nil {
		return "", false, errors.New("replay agent is nil")
	}
	return replayer.agent.QueryAction(ctx, protocol, request, ExactMatch)
}

func BestQuery(ctx context.Context, protocol, request string) (response string, ok bool, err error) {
	if replayer.agent == nil {
		return "", false, errors.New("replay agent is nil")
	}
	return replayer.agent.QueryAction(ctx, protocol, request, BestMatch)
}
