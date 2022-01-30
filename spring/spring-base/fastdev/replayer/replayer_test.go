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

package replayer_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/replayer"
	"github.com/go-spring/spring-base/knife"
)

func init() {
	fastdev.RegisterProtocol(fastdev.HTTP, &httpProtocol{})
	fastdev.RegisterProtocol(fastdev.REDIS, &redisProtocol{})
}

func TestReplayAction(t *testing.T) {

	replayer.SetReplayMode(true)
	defer func() {
		replayer.SetReplayMode(false)
	}()

	sessionID := "39fc5c13443f47da9ff320cc4b02c789"
	ctx, _ := knife.New(context.Background())
	err := replayer.SetSessionID(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	recordSession := &fastdev.Session{
		Session:   sessionID,
		Timestamp: chrono.Now(ctx).UnixNano(),
		Inbound: &fastdev.Action{
			Protocol:  fastdev.HTTP,
			Timestamp: chrono.Now(ctx).UnixNano(),
			Request:   []interface{}{"GET", "..."},
			Response:  []interface{}{200, "..."},
		},
		Actions: []*fastdev.Action{
			{
				Protocol:  fastdev.REDIS,
				Timestamp: chrono.Now(ctx).UnixNano(),
				Request:   []interface{}{"SET", "a", "1"},
				Response:  []interface{}{1, "2", 3},
			}, {
				Protocol:  fastdev.REDIS,
				Timestamp: chrono.Now(ctx).UnixNano(),
				Request:   []interface{}{"SET", "a", "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"},
				Response:  []interface{}{"\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"},
			},
			{
				Protocol:  fastdev.REDIS,
				Timestamp: chrono.Now(ctx).UnixNano(),
				Request:   []interface{}{"HGET", "a"},
				Response: map[string]interface{}{
					"a": "b",
					"c": 3,
					"d": "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
				},
			},
		},
	}

	str, err := recordSession.Pretty()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("encode:", str)

	session, err := fastdev.ToSession(str)
	if err != nil {
		t.Fatal(err)
	}

	err = replayer.Store(session)
	if err != nil {
		t.Fatal(err)
	}

	{
		query := &fastdev.Action{
			Protocol: fastdev.REDIS,
			Request:  []interface{}{"SET", "a", "1"},
		}

		var action *fastdev.Action
		action, err = replayer.GetAction(ctx, query)
		assert.Nil(t, err)
		assert.NotNil(t, action)

		fmt.Print("action: ")
		fmt.Println(action.Pretty())
	}

	{
		query := &fastdev.Action{
			Protocol: fastdev.REDIS,
			Request:  []interface{}{"SET", "a", "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"},
		}

		var action *fastdev.Action
		action, err = replayer.GetAction(ctx, query)
		assert.Nil(t, err)
		assert.NotNil(t, action)

		fmt.Print("action: ")
		fmt.Println(action.Pretty())
	}
}

type httpProtocol struct{}

func (p *httpProtocol) GetLabel(req interface{}) string {
	r := req.([]interface{})
	return r[0].(string) + "@" + strings.SplitN(r[1].(string), "?", 2)[0]
}

type redisProtocol struct{}

func (p *redisProtocol) GetLabel(req interface{}) string {
	r := req.([]interface{})
	return r[0].(string)
}
