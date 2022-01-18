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
			ID:       2,
			Protocol: fastdev.HTTP,
			Request:  "GET ...",
			Response: "... 200 ...",
		},
		Actions: []*recorder.Action{
			{
				ID:       0,
				Protocol: fastdev.REDIS,
				Request:  "SET a 1",
				Response: "OK",
			}, {
				ID:       1,
				Protocol: fastdev.REDIS,
				Request:  "GET a",
				Response: int64(1),
			},
		},
	}

	data, err := recorder.ToJson(recordSession)
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

	action := &replayer.Action{
		Protocol: fastdev.REDIS,
		Request:  "GET a",
	}

	ok, err := replayer.GetAction(ctx, action)
	assert.Nil(t, err)
	assert.True(t, ok)

	var i int64
	err = action.Response.ToValue(&i)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, i, int64(1))
}
