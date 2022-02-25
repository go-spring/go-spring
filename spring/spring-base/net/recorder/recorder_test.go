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

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/net/recorder"
)

func TestRecordAction(t *testing.T) {

	recorder.SetRecordMode(true)
	defer func() {
		recorder.SetRecordMode(false)
	}()

	timeNow := time.Unix(1643364150, 0)
	ctx, _ := knife.New(context.Background())
	err := clock.SetBaseTime(ctx, timeNow)
	if err != nil {
		t.Fatal(err)
	}

	sessionID := "df3b64266ebe4e63a464e135000a07cd"
	ctx = recorder.StartRecord(ctx, sessionID)

	recorder.RecordAction(ctx, &recorder.Action{
		Protocol: recorder.REDIS,
		Request: recorder.NewMessage(func() string {
			return cast.ToCommandLine("SET", "a", "\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n")
		}),
		Response: recorder.NewMessage(func() string {
			return cast.ToCSV("\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n")
		}),
	})

	recorder.RecordAction(ctx, &recorder.Action{
		Protocol: recorder.REDIS,
		Request: recorder.NewMessage(func() string {
			return cast.ToCommandLine("LRANGE", "list", 0, -1)
		}),
		Response: recorder.NewMessage(func() string {
			return cast.ToCSV("1", 2, "3")
		}),
	})

	recorder.RecordInbound(ctx, &recorder.Action{
		Protocol: recorder.HTTP,
		Request: recorder.NewMessage(func() string {
			return "GET ..."
		}),
		Response: recorder.NewMessage(func() string {
			return "200 ..."
		}),
	})

	s := recorder.StopRecord(ctx)
	str, err := s.PrettyJson()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("got:", str)

	s1, err := recorder.ToRawSession(str)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Print("json(got): ")
	fmt.Println(s1.PrettyJson())

	expect := `{
	  "Session": "df3b64266ebe4e63a464e135000a07cd",
	  "Timestamp": 1643364150000040916,
	  "Inbound": {
		"Protocol": "HTTP",
		"Timestamp": 1643364150000045348,
		"Request": "GET ...",
		"Response": "200 ..."
	  },
	  "Actions": [
		{
		  "Protocol": "REDIS",
		  "Timestamp": 1643364150000040916,
		  "Request": "SET a \"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\"",
		  "Response": "\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\""
		},
		{
		  "Protocol": "REDIS",
		  "Timestamp": 1643364150000040916,
		  "Request": "LRANGE list 0 -1",
		  "Response": "\"1\",\"2\",\"3\""
		}
	  ]
	}`

	s2, err := recorder.ToRawSession(expect)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Print("json(expect): ")
	fmt.Println(s2.PrettyJson())
}
