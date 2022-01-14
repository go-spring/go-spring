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

package record

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/redis/test/cases"
)

func RunCase(t *testing.T, d redis.Driver, c cases.Case) {

	fastdev.SetRecordMode(true)
	defer func() {
		fastdev.SetRecordMode(false)
	}()

	ctx, _ := knife.New(context.Background())
	sessionID := "df3b64266ebe4e63a464e135000a07cd"
	err := knife.Set(ctx, fastdev.RecordSessionIDKey, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	config := redis.ClientConfig{Port: 6379}
	client, err := redis.NewClient(config, d)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		fastdev.SetRecordMode(false)
		client.FlushAll(ctx)
	}()

	c.Func(t, ctx, client)

	session := fastdev.RecordInbound(ctx, &fastdev.Action{})
	if c.Data != "skip" {
		testResult := session.ToJson()

		var (
			s1 *fastdev.Session
			s2 *fastdev.Session
		)

		s1, err = fastdev.ToSession([]byte(testResult), true)
		if err != nil {
			t.Fatal(err)
		}

		s2, err = fastdev.ToSession([]byte(c.Data), true)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(s1, s2) {
			fail(t, 0, "got %v but expect %v", testResult, c.Data)
		}
	}
}

func fail(t *testing.T, skip int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	_, file, line, _ := runtime.Caller(skip + 1)
	fmt.Printf("\t%s:%d: %s\n", filepath.Base(file), line, msg)
	t.Fail()
}
