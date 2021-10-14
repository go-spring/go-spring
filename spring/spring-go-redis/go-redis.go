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

	g "github.com/go-redis/redis/v8"
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
	_, err := cmd.Result()
	if err != nil {
		if err == g.Nil {
			return nil, redis.ErrNil
		}
		return nil, err
	}
	return &reply{cmd: cmd}, nil
}

type reply struct {
	cmd *g.Cmd
}

func (r *reply) Bool() bool {
	switch v := r.cmd.Val().(type) {
	case int64:
		return v == 1
	case string:
		return v == "OK"
	default:
		return false
	}
}

func (r *reply) Int64() int64 {
	val, _ := r.cmd.Int64()
	return val
}

func (r *reply) Float64() float64 {
	val, _ := r.cmd.Float64()
	return val
}

func (r *reply) String() string {
	val, _ := r.cmd.Text()
	return val
}

func (r *reply) Slice() []interface{} {
	val, _ := r.cmd.Slice()
	return val
}

func (r *reply) Int64Slice() []int64 {
	val, _ := r.cmd.Int64Slice()
	return val
}

func (r *reply) Float64Slice() []float64 {
	val, _ := r.cmd.Float64Slice()
	return val
}

func (r *reply) BoolSlice() []bool {
	val, _ := r.cmd.BoolSlice()
	return val
}

func (r *reply) StringSlice() []string {
	val, _ := r.cmd.StringSlice()
	return val
}

func (r *reply) StringStringMap() map[string]string {
	ss, _ := r.cmd.StringSlice()
	val := make(map[string]string, len(ss)/2)
	for i := 0; i < len(ss); i += 2 {
		val[ss[i]] = ss[i+1]
	}
	return val
}
