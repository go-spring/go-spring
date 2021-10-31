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
	"fmt"
	"strconv"

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

func toBool(val interface{}) (bool, error) {
	switch v := val.(type) {
	case int64:
		return v == 1, nil
	case string:
		return v == "OK", nil
	default:
		return false, fmt.Errorf("redis: unexpected type %T for bool", v)
	}
}

func (r *reply) Value() interface{} {
	return r.v
}

func (r *reply) Bool() (bool, error) {
	return toBool(r.v)
}

func (r *reply) Int64() (int64, error) {
	return g.Int64(r.v, nil)
}

func (r *reply) Float64() (float64, error) {
	return g.Float64(r.v, nil)
}

func (r *reply) String() (string, error) {
	return g.String(r.v, nil)
}

func (r *reply) Slice() ([]interface{}, error) {
	val, err := g.Values(r.v, nil)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(val); i++ {
		switch v := val[i].(type) {
		case []byte:
			val[i] = string(v)
		}
	}
	return val, nil
}

func (r *reply) BoolSlice() ([]bool, error) {
	slice, err := g.Values(r.v, nil)
	if err != nil {
		return nil, err
	}
	val := make([]bool, len(slice))
	for i := 0; i < len(slice); i++ {
		if val[i], err = toBool(slice[i]); err != nil {
			return nil, err
		}
	}
	return val, nil
}

func (r *reply) Int64Slice() ([]int64, error) {
	return g.Int64s(r.v, nil)
}

func (r *reply) Float64Slice() ([]float64, error) {
	return g.Float64s(r.v, nil)
}

func (r *reply) StringSlice() ([]string, error) {
	return g.Strings(r.v, nil)
}

func (r *reply) ZItemSlice() ([]redis.ZItem, error) {
	slice, err := g.Values(r.v, nil)
	if err != nil {
		return nil, err
	}
	val := make([]redis.ZItem, len(slice)/2)
	for i := 0; i < len(val); i++ {
		idx := 2 * i
		member, ok := slice[idx].([]uint8)
		if !ok {
			return nil, fmt.Errorf("redis: unexpected type %T for string", slice[i])
		}
		score, ok := slice[idx+1].([]uint8)
		if !ok {
			return nil, fmt.Errorf("redis: unexpected type %T for float64", slice[i+1])
		}
		score0, err := strconv.ParseFloat(string(score), 64)
		if err != nil {
			return nil, err
		}
		val[i] = redis.ZItem{Score: score0, Member: string(member)}
	}
	return val, nil
}

func (r *reply) StringMap() (map[string]string, error) {
	return g.StringMap(r.v, nil)
}
