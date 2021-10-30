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
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/knife"
)

func TestRecordAction(t *testing.T) {

	recordMode = true
	defer func() {
		recordMode = false
	}()

	ctx := knife.New(context.Background())
	err := knife.Set(ctx, RecordSessionIDKey, NewSessionID())
	if err != nil {
		t.Fatal(err)
	}

	RecordAction(ctx, &Action{
		Protocol: REDIS,
		Request:  "GET a",
		Response: "1",
	})

	session := RecordInbound(ctx, &Action{
		Protocol: HTTP,
		Request:  "GET ...",
		Response: "... 200 ...",
	})

	_, ok := recordData.Load(session.Session)
	assert.False(t, ok)
}
