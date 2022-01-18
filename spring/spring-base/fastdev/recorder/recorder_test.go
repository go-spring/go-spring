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

	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/fastdev/recorder"
	"github.com/go-spring/spring-base/knife"
)

func TestRecordAction(t *testing.T) {

	recorder.SetRecordMode(true)
	defer func() {
		recorder.SetRecordMode(false)
	}()

	ctx, _ := knife.New(context.Background())
	_, err := recorder.StartRecord(ctx)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		var session *recorder.Session
		session, err = recorder.StopRecord(ctx)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(recorder.ToPrettyJson(session, "  "))
	}()

	err = recorder.RecordAction(ctx, &recorder.Action{
		Protocol: fastdev.REDIS,
		Request:  "GET a",
		Response: int64(1),
	})
	if err != nil {
		t.Fatal(err)
	}

	err = recorder.RecordInbound(ctx, &recorder.Action{
		Protocol: fastdev.HTTP,
		Request:  "GET ...",
		Response: "... 200 ...",
	})
	if err != nil {
		t.Fatal(err)
	}
}
