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
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/knife"
)

var (
	recordMode bool     // 是否为录制模式。
	recordData sync.Map // 正在录制的数据。
)

func init() {
	e := os.Getenv("fastdev_mode")
	ss := strings.Split(e, ",")
	for _, s := range ss {
		if s == "record" {
			recordMode = true
			break
		}
	}
}

// RecordMode 返回是否是录制模式。
func RecordMode() bool {
	return recordMode
}

func checkRecordMode() {
	if !recordMode {
		panic(errors.New("record mode not enabled"))
	}
}

type recordSession struct {
	s *Session
	m sync.Mutex
}

func getRecordSession(ctx context.Context) *recordSession {

	var sessionID string

	if v, ok := knife.Get(ctx, RecordSessionIDKey); !ok {
		panic(errors.New("session id not found"))
	} else {
		if sessionID, ok = v.(string); !ok {
			panic(errors.New("session id not string"))
		}
	}

	v, ok := recordData.Load(sessionID)
	if ok {
		return v.(*recordSession)
	}

	s := &recordSession{s: &Session{Session: sessionID}}
	actual, _ := recordData.LoadOrStore(sessionID, s)
	return actual.(*recordSession)
}

// RecordAction 录制一个动作，会话 ID 从 Context 对象中获取。
func RecordAction(ctx context.Context, action *Action) {

	checkRecordMode()
	s := getRecordSession(ctx)

	s.m.Lock()
	defer s.m.Unlock()

	s.s.Actions = append(s.s.Actions, action)
}

// RecordInbound 录制上游流量，会话 ID 从 Context 对象中获取。
func RecordInbound(ctx context.Context, inbound *Action) *Session {

	checkRecordMode()
	s := getRecordSession(ctx)

	defer func() {
		recordData.Delete(s.s.Session)
	}()

	s.m.Lock()
	defer s.m.Unlock()

	s.s.Inbound = inbound
	fmt.Println(cast.ToString(s.s))
	return s.s
}
