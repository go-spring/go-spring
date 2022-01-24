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
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/recorder"
	"github.com/go-spring/spring-base/fastdev/replayer"
	"github.com/go-spring/spring-base/knife"
)

func TestReplayAction(t *testing.T) {

	replayer.SetReplayMode(true)
	defer func() {
		replayer.SetReplayMode(false)
	}()

	sessionID := fastdev.NewSessionID()
	ctx, _ := knife.New(context.Background())
	err := replayer.SetSessionID(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	recordSession := &recorder.Session{
		Session: sessionID,
		Inbound: &recorder.Action{
			Protocol: fastdev.HTTP,
			Request:  recorder.NewMessage("GET ..."),
			Response: recorder.NewMessage("... 200 ..."),
		},
		Actions: []*recorder.Action{
			{
				Protocol: fastdev.REDIS,
				Request:  recorder.NewMessage("SET a 1"),
				Response: recorder.NewMessage([]interface{}{1, "2", 3}),
			}, {
				Protocol: fastdev.REDIS,
				Request:  recorder.NewMessage("GET a"),
				Response: recorder.NewMessage("\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"),
			},
			{
				Protocol: fastdev.REDIS,
				Request:  recorder.NewMessage("HGET a"),
				Response: recorder.NewMessage(map[string]interface{}{
					"a": "b",
					"c": 3,
					"d": "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n",
				}),
			},
		},
	}

	data, err := recorder.ToPrettyJson(recordSession)
	if err != nil {
		t.Fatal(err)
	}

	session, err := replayer.ToSession(data)
	if err != nil {
		t.Fatal(err)
	}

	err = replayer.Store(session)
	if err != nil {
		t.Fatal(err)
	}

	query := &recorder.Action{
		Protocol: fastdev.REDIS,
		Request:  recorder.NewMessage("GET a"),
	}

	var action *replayer.Action
	action, err = replayer.GetAction(ctx, query)
	assert.Nil(t, err)
	assert.NotNil(t, action)

	var i string
	err = action.Response.ToValue(&i)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%#v %T\n", i, i)
}
