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

package recorder_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/chrono"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/recorder"
	"github.com/go-spring/spring-base/knife"
)

func TestRecordAction(t *testing.T) {

	recorder.SetRecordMode(true)
	defer func() {
		recorder.SetRecordMode(false)
	}()

	timeNow := time.Unix(1643364150, 0)
	ctx, _ := knife.New(context.Background())
	err := chrono.SetBaseTime(ctx, timeNow)
	if err != nil {
		t.Fatal(err)
	}

	sessionID := "df3b64266ebe4e63a464e135000a07cd"
	err = recorder.StartRecord(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.RecordAction(ctx, &fastdev.Action{
		Protocol: fastdev.REDIS,
		Request:  []interface{}{"SET", "a", "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"},
		Response: []interface{}{"\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.RecordAction(ctx, &fastdev.Action{
		Protocol: fastdev.REDIS,
		Request:  []interface{}{"LRANGE", "list", 0, -1},
		Response: []interface{}{"1", 2, "3"},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.RecordInbound(ctx, &fastdev.Action{
		Protocol: fastdev.HTTP,
		Request:  []interface{}{"GET", "..."},
		Response: []interface{}{200, "..."},
	})
	if err != nil {
		t.Fatal(err)
	}

	s, err := recorder.StopRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}

	str, err := s.Pretty()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("encode:", str)

	s1, err := fastdev.ToSession(str)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Print("decode: ")
	fmt.Println(s1.Pretty())

	expect := `{
	  "session": "df3b64266ebe4e63a464e135000a07cd",
	  "timestamp": 1643364150000047753,
	  "inbound": {
		"protocol": "HTTP",
		"timestamp": 1643364150000054648,
		"request": ["GET", "..."],
		"response": [200, "..."]
	  },
	  "actions": [
		{
		  "protocol": "REDIS",
		  "timestamp": 1643364150000047753,
		  "request": ["SET", "a", "@\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\""],
		  "response": ["@\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\""]
		},
		{
		  "protocol": "REDIS",
		  "timestamp": 1643364150000047753,
		  "request": ["LRANGE", "list", 0, -1],
		  "response": ["1", 2, "3"]
		}
	  ]
	}`

	s2, err := fastdev.ToSession(expect)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Print("expect: ")
	fmt.Println(s2.Pretty())

	eq, err := fastdev.DiffSession(s1, s2, []string{})
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, eq)
}
