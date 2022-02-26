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

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/net/replayer"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/redis/test/cases"
)

type driver struct{}

func (d *driver) Open(config redis.Config) (redis.Conn, error) {
	return &conn{}, nil
}

type conn struct{}

func (c *conn) Exec(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
	panic(util.ForbiddenMethod)
}

func RunCase(t *testing.T, c cases.Case) {

	replayer.SetReplayMode(true)
	defer func() {
		replayer.SetReplayMode(false)
	}()

	agent := replayer.NewLocalAgent()
	replayer.SetReplayAgent(agent)

	session, err := agent.Store(c.Data)
	assert.Nil(t, err)

	ctx, _ := knife.New(context.Background())
	err = replayer.SetSessionID(ctx, session.Session)
	if err != nil {
		t.Fatal(err)
	}

	config := redis.Config{Port: 6379}
	client, err := redis.NewClient(config, &driver{})
	if err != nil {
		t.Fatal(err)
	}

	c.Func(t, ctx, client)
}
