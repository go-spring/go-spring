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

package testcases

import (
	"context"
	"testing"

	g "github.com/go-redis/redis/v8"
	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-core/redis"
	SpringGoRedis "github.com/go-spring/spring-go-redis"
)

func RunCase(t *testing.T,
	testFunc func(t *testing.T, ctx context.Context, c redis.Client),
	recordResult string) {

	fastdev.SetRecordMode(true)
	defer func() {
		fastdev.SetRecordMode(false)
	}()

	ctx := knife.New(context.Background())
	sessionID := "df3b64266ebe4e63a464e135000a07cd"
	err := knife.Set(ctx, fastdev.RecordSessionIDKey, sessionID)
	if err != nil {
		t.Fatal(err)
	}

	c := SpringGoRedis.NewClient(g.NewClient(&g.Options{}))

	defer func() {
		fastdev.SetRecordMode(false)
		c.FlushAll(ctx)
	}()

	testFunc(t, ctx, c)

	session := fastdev.RecordInbound(ctx, &fastdev.Action{})
	if recordResult != `skip` {
		assert.Equal(t, cast.ToString(session), recordResult)
	}
}
