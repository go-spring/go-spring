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

package SpringGoRedis

import (
	"context"
	"time"

	g "github.com/go-redis/redis/v8"
	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-core/redis"
)

type client struct {
	redis.BaseClient
	client *g.Client
}

func NewClient(clt *g.Client) redis.Client {
	c := &client{client: clt}
	c.DoFunc = c.do
	return c
}

func (c *client) do(ctx context.Context, args ...interface{}) (redis.Reply, error) {
	cmd := c.client.Do(ctx, args...)
	result, err := cmd.Result()
	if err != nil {
		if err == g.Nil {
			return nil, redis.ErrNil
		}
		return nil, err
	}
	return &reply{v: result}, nil
}

type reply struct {
	v interface{}
}

func (r *reply) Bool() bool {
	return cast.ToBool(r.v)
}

func (r *reply) Int64() int64 {
	return cast.ToInt64(r.v)
}

func (r *reply) Float64() float64 {
	return cast.ToFloat64(r.v)
}

func (r *reply) String() string {
	return cast.ToString(r.v)
}

func (r *reply) Duration() time.Duration {
	return cast.ToDuration(r.v)
}

func (r *reply) Slice() []interface{} {
	return nil
}

func (r *reply) Int64Slice() []int64 {
	return nil
}

func (r *reply) BoolSlice() []bool {
	return nil
}

func (r *reply) StringSlice() []string {
	return cast.ToStringSlice(r.v)
}

func (r *reply) StringStringMap() map[string]string {
	return nil
}
