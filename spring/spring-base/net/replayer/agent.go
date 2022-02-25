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
	"sync"

	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/net/recorder"
)

type MatchStrategy int

const (
	ExactMatch MatchStrategy = iota
	BestMatch
)

type Agent interface {
	QueryAction(ctx context.Context, protocol, request string, matchStrategy MatchStrategy) (response string, ok bool, err error)
}

type LocalAgent struct {
	data sync.Map
}

type replayData struct {
	session *Session
	matched sync.Map
	actions map[string]map[string][]*Action
}

func NewLocalAgent() *LocalAgent {
	return &LocalAgent{}
}

// Store 存储 sessionID 对应的回放数据。
func (agent *LocalAgent) Store(str string) (*Session, error) {
	rawSession, err := recorder.ToRawSession(str)
	if err != nil {
		return nil, err
	}
	session, err := ToSession(rawSession)
	if err != nil {
		return nil, err
	}
	err = agent.store(session)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (agent *LocalAgent) store(session *Session) error {

	actions := make(map[string]map[string][]*Action)
	for _, a := range session.Actions {
		p := recorder.GetProtocol(a.Protocol)
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

	data := &replayData{session: session, actions: actions}
	_, loaded := agent.data.LoadOrStore(session.Session, data)
	if loaded {
		return errors.New("session already stored")
	}
	return nil
}

// Delete 删除 sessionID 对应的回放数据。
func (agent *LocalAgent) Delete(sessionID string) error {
	_, ok := agent.data.Load(sessionID)
	if !ok {
		return errors.New("session already deleted")
	}
	agent.data.Delete(sessionID)
	return nil
}

func (agent *LocalAgent) getReplayData(ctx context.Context) (*replayData, error) {
	sessionID, err := GetSessionID(ctx)
	if err != nil {
		return nil, err
	}
	v, ok := agent.data.Load(sessionID)
	if !ok {
		return nil, errors.New("session not found")
	}
	return v.(*replayData), nil
}

func (agent *LocalAgent) QueryAction(ctx context.Context, protocol, request string, matchStrategy MatchStrategy) (response string, ok bool, err error) {

	r, err := agent.getReplayData(ctx)
	if err != nil {
		return "", false, err
	}

	p := recorder.GetProtocol(protocol)
	if p == nil {
		return "", false, errors.New("invalid protocol")
	}

	m, ok := r.actions[protocol]
	if !ok {
		return "", false, nil
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
		action.RecTimestamp = clock.Now(ctx).UnixNano()
		return action.Response, true, nil
	}
	return "", false, nil
}

type RemoteAgent struct {
}

func (agent *RemoteAgent) QueryAction(ctx context.Context, protocol, request string, matchStrategy MatchStrategy) (response string, ok bool, err error) {
	return "", false, nil
}
