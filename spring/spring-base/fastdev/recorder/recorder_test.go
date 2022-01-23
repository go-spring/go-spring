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
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/recorder"
	"github.com/go-spring/spring-base/knife"
)

func TestRecordAction(t *testing.T) {

	recorder.SetRecordMode(true)
	defer func() {
		recorder.SetRecordMode(false)
	}()

	sessionID := "df3b64266ebe4e63a464e135000a07cd"
	ctx, _ := knife.New(context.Background())
	err := recorder.StartRecord(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.RecordAction(ctx, &recorder.Action{
		Protocol: fastdev.REDIS,
		Request:  recorder.NewMessage("SET a \"\\u0000\\xC0\\n\\t\\u0000\\xBEm\\u0006\\x89Z(\\u0000\\n\""),
		Response: recorder.NewMessage("\x00\xc0\n\t\x00\xbem\x06\x89Z(\x00\n"),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.RecordInbound(ctx, &recorder.Action{
		Protocol: fastdev.HTTP,
		Request:  recorder.NewMessage("GET ..."),
		Response: recorder.NewMessage("... 200 ..."),
	})
	if err != nil {
		t.Fatal(err)
	}

	s, err := recorder.StopRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}

	str, err := recorder.ToPrettyJson(s)
	if err != nil {
		t.Fatal(err)
	}

	assert.JsonEqual(t, str, `{
	  "session": "df3b64266ebe4e63a464e135000a07cd",
	  "inbound": {
		"protocol": "HTTP",
		"request": "GET ...",
		"response": "... 200 ...",
		"timestamp": 0
	  },
	  "actions": [
		{
		  "protocol": "REDIS",
		  "request": "SET a \"\\u0000\\xC0\\n\\t\\u0000\\xBEm\\u0006\\x89Z(\\u0000\\n\"",
		  "response": "@\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\"",
		  "timestamp": 0
		}
	  ]
	}`)
}
