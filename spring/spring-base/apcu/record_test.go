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

package apcu_test

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/apcu"
	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/knife"
)

func TestRecord(t *testing.T) {

	if !fastdev.RecordMode() {
		t.SkipNow()
	}

	sessionID := fastdev.NewSessionID()
	ctx := knife.New(context.Background())
	err := knife.Set(ctx, fastdev.RecordSessionIDKey, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	type dataType struct {
		Data string `json:"a"`
	}

	var a = dataType{
		Data: "success",
	}

	apcu.Store("a", a)

	var b dataType
	ok, err := apcu.Load(ctx, "a", &b)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, ok)

	session := fastdev.RecordInbound(ctx, &fastdev.Action{
		Protocol: fastdev.HTTP,
		Request:  "GET ...",
		Response: "... 200 ...",
	})

	assert.Equal(t, session.Session, sessionID)
}
