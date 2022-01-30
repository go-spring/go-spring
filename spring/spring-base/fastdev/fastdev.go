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
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev/internal/json"
	"github.com/google/uuid"
)

const (
	HTTP  = "HTTP"
	SQL   = "SQL"
	REDIS = "REDIS"
	APCU  = "APCU"
)

var (
	protocols = map[string]Protocol{}
)

type Protocol interface {
	GetLabel(request interface{}) string
}

func GetProtocol(name string) Protocol {
	return protocols[name]
}

func RegisterProtocol(name string, protocol Protocol) {
	if _, ok := protocols[name]; ok {
		panic(fmt.Errorf("%s: duplicate registration", name))
	}
	protocols[name] = protocol
}

// NewSessionID 使用 uuid 算法生成新的 Session ID 。
func NewSessionID() string {
	u := uuid.New()
	buf := make([]byte, 32)
	hex.Encode(buf, u[:4])
	hex.Encode(buf[8:12], u[4:6])
	hex.Encode(buf[12:16], u[6:8])
	hex.Encode(buf[16:20], u[8:10])
	hex.Encode(buf[20:], u[10:])
	return string(buf)
}

// CheckTestMode 检查是否是测试模式
func CheckTestMode() {
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "-test.") {
			return
		}
	}
	panic(errors.New("must call under test mode"))
}

type Session struct {
	Session   string    `json:"session,omitempty"`   // 会话 ID
	Timestamp int64     `json:"timestamp,omitempty"` // 时间戳
	Inbound   *Action   `json:"inbound,omitempty"`   // 上游数据
	Actions   []*Action `json:"actions,omitempty"`   // 动作数据
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
	Protocol     string                 `json:"protocol,omitempty"`     // 协议名称
	Timestamp    int64                  `json:"timestamp,omitempty"`    // 时间戳
	Request      interface{}            `json:"request,omitempty"`      // 请求内容
	Response     interface{}            `json:"response,omitempty"`     // 响应内容
	FlatRequest  map[string]interface{} `json:"flatRequest,omitempty"`  // 请求内容
	FlatResponse map[string]interface{} `json:"flatResponse,omitempty"` // 响应内容
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

type rawSession struct {
	Session   string       `json:"session,omitempty"`   // 会话 ID
	Inbound   *rawAction   `json:"inbound,omitempty"`   // 上游数据
	Actions   []*rawAction `json:"actions,omitempty"`   // 动作数据
	Timestamp int64        `json:"timestamp,omitempty"` // 时间戳
}

type rawAction struct {
	Protocol  string          `json:"protocol,omitempty"`  // 协议名称
	Request   json.RawMessage `json:"request,omitempty"`   // 请求内容
	Response  json.RawMessage `json:"response,omitempty"`  // 响应内容
	Timestamp int64           `json:"timestamp,omitempty"` // 时间戳
}

func (r *rawAction) ToAction() (*Action, error) {
	var req interface{}
	if err := json.Unmarshal(r.Request, &req); err != nil {
		return nil, err
	}
	var resp interface{}
	if err := json.Unmarshal(r.Response, &resp); err != nil {
		return nil, err
	}
	return &Action{
		Protocol:     r.Protocol,
		Timestamp:    r.Timestamp,
		Request:      req,
		Response:     resp,
		FlatRequest:  cast.FlatBytes(r.Request),
		FlatResponse: cast.FlatBytes(r.Response),
	}, nil
}

func ToSession(data string) (*Session, error) {
	var r *rawSession
	err := json.Unmarshal([]byte(data), &r)
	if err != nil {
		return nil, err
	}
	inbound, err := r.Inbound.ToAction()
	if err != nil {
		return nil, err
	}
	var actions []*Action
	for _, a := range r.Actions {
		var c *Action
		c, err = a.ToAction()
		if err != nil {
			return nil, err
		}
		actions = append(actions, c)
	}
	return &Session{
		Session:   r.Session,
		Inbound:   inbound,
		Actions:   actions,
		Timestamp: r.Timestamp,
	}, nil
}

func Equal(b1 string, b2 string, ignores []string) (bool, error) {

	s1, err := ToSession(b1)
	if err != nil {
		return false, err
	}
	fmt.Println(s1.Pretty())

	s2, err := ToSession(b2)
	if err != nil {
		return false, err
	}
	fmt.Println(s2.Pretty())

	return DiffSession(s1, s2, ignores)
}

func DiffSession(s1 *Session, s2 *Session, ignores []string) (bool, error) {
	if s1.Session != s2.Session {
		return false, errors.New("session id not equal")
	}
	if n := s1.Timestamp - s2.Timestamp; n > int64(time.Second) || n < -int64(time.Second) {
		return false, errors.New("timestamp not equal")
	}
	_, err := diffAction(s1.Inbound, s2.Inbound, ignores)
	if err != nil {
		return false, err
	}
	if len(s1.Actions) != len(s2.Actions) {
		return false, errors.New("actions not equal")
	}
	for i := 0; i < len(s1.Actions); i++ {
		_, err = diffAction(s1.Actions[i], s2.Actions[i], ignores)
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func diffAction(a1 *Action, a2 *Action, ignores []string) (bool, error) {
	if a1.Protocol != a2.Protocol {
		return false, errors.New("protocol not equal")
	}
	if n := a1.Timestamp - a2.Timestamp; n > int64(time.Second) || n < -int64(time.Second) {
		return false, errors.New("timestamp not equal")
	}
	if !reflect.DeepEqual(a1.Request, a2.Request) {
		return false, errors.New("request not equal")
	}
	if !reflect.DeepEqual(a1.Response, a2.Response) {
		return false, errors.New("response not equal")
	}
	return true, nil
}
