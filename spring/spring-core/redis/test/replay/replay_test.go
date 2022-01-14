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

package replay

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/redis/test/cases"
)

func RunCase(t *testing.T, c cases.Case) {

	fastdev.SetReplayMode(true, false)
	defer func() {
		fastdev.SetReplayMode(false, false)
	}()

	session, err := fastdev.ToSession([]byte(c.Data), false)
	if err != nil {
		t.Fatal(err)
	}

	ctx, _ := knife.New(context.Background())
	err = knife.Set(ctx, fastdev.ReplaySessionIDKey, session.Session)
	if err != nil {
		t.Fatal(err)
	}

	fastdev.Store(session)
	defer fastdev.Delete(session.Session)

	config := redis.ClientConfig{Port: 6379}
	client, err := redis.NewClient(config, nil)
	if err != nil {
		t.Fatal(err)
	}

	c.Func(t, ctx, client)
}
