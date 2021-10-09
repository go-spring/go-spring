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

package SpringRedigo

import (
	"context"

	"github.com/go-spring/spring-core/redis"
	g "github.com/gomodule/redigo/redis"
)

type client struct {
	redis.BaseClient
	conn g.Conn
}

func NewClient(conn g.Conn) redis.Client {
	c := &client{conn: conn}
	c.DoFunc = c.do
	return c
}

func (c *client) do(ctx context.Context, args ...interface{}) (redis.Reply, error) {
	result, err := c.conn.Do(args[0].(string), args[1:]...)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, redis.ErrNil
	}
	return &reply{v: result}, nil
}

type reply struct {
	v interface{}
}

func (r *reply) Bool() bool {
	switch v := r.v.(type) {
	case int64:
		return v == 1
	case string:
		return v == "OK"
	default:
		return false
	}
}

func (r *reply) Int64() int64 {
	val, _ := g.Int64(r.v, nil)
	return val
}

func (r *reply) Float64() float64 {
	val, _ := g.Float64(r.v, nil)
	return val
}

func (r *reply) String() string {
	val, _ := g.String(r.v, nil)
	return val
}

func (r *reply) Slice() []interface{} {
	val, _ := g.Values(r.v, nil)
	for i := 0; i < len(val); i++ {
		switch v := val[i].(type) {
		case []byte:
			val[i] = string(v)
		}
	}
	return val
}

func (r *reply) Int64Slice() []int64 {
	val, _ := g.Int64s(r.v, nil)
	return val
}

func (r *reply) BoolSlice() []bool {
	//val, _ := g.Bool(r.v, nil)
	//return val
	return nil
}

func (r *reply) StringSlice() []string {
	val, _ := g.Strings(r.v, nil)
	return val
}

func (r *reply) StringStringMap() map[string]string {
	val, _ := g.StringMap(r.v, nil)
	return val
}
