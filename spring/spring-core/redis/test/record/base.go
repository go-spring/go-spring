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
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/clock"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/net/recorder"
	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/redis/test/cases"
)

func flushAll(d redis.Driver) (string, error) {
	config := redis.Config{Port: 6379}
	c, err := redis.NewClient(config, d)
	if err != nil {
		return "", err
	}
	return c.FlushAll(context.Background())
}

func RunCase(t *testing.T, d redis.Driver, c cases.Case) {

	ok, err := flushAll(d)
	assert.Nil(t, err)
	assert.True(t, redis.IsOK(ok))

	recorder.SetRecordMode(true)
	defer func() {
		recorder.SetRecordMode(false)
	}()

	config := redis.Config{Port: 6379}
	client, err := redis.NewClient(config, d)
	assert.Nil(t, err)

	ctx, _ := knife.New(context.Background())
	err = clock.SetFixedTime(ctx, time.Unix(0, 0))
	assert.Nil(t, err)

	sessionID := "df3b64266ebe4e63a464e135000a07cd"
	recorder.StartRecord(ctx, sessionID)

	c.Func(t, ctx, client)

	session := recorder.StopRecord(ctx)
	if c.Skip {
		return
	}

	str := recorder.ToPrettyJson(session)
	assert.JsonEqual(t, str, c.Data)
}
