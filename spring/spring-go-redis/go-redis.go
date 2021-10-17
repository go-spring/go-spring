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
	"fmt"
	"strconv"

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

func (r *reply) Bool() (bool, error) {
	switch v := r.cmd.Val().(type) {
	case int64:
		return v == 1, nil
	case string:
		return v == "OK", nil
	default:
		return false, fmt.Errorf("redis: unexpected type=%T for bool", v)
	}
}

func (r *reply) Int64() (int64, error) {
	return r.cmd.Int64()
}

func (r *reply) Float64() (float64, error) {
	return r.cmd.Float64()
}

func (r *reply) String() (string, error) {
	return r.cmd.Text()
}

func (r *reply) Slice() ([]interface{}, error) {
	return r.cmd.Slice()
}

func (r *reply) BoolSlice() ([]bool, error) {
	return r.cmd.BoolSlice()
}

func (r *reply) Int64Slice() ([]int64, error) {
	return r.cmd.Int64Slice()
}

func (r *reply) Float64Slice() ([]float64, error) {
	slice, err := r.Slice()
	if err != nil {
		return nil, err
	}

	floats := make([]float64, 0)
	for _, iface := range slice {
		// 当 redis 返回 <nil>，跳出本次循环
		if iface == nil {
			continue
		}

		val, err := toFloat64(iface)
		if err != nil {
			return nil, err
		}
		floats = append(floats, val)
	}
	return floats, nil
}

func toFloat64(val interface{}) (float64, error) {
	switch val := val.(type) {
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		err := fmt.Errorf("redis: unexpected type=%T for Float64", val)
		return 0, err
	}
}

func (r *reply) StringSlice() ([]string, error) {
	return r.cmd.StringSlice()
}

func (r *reply) ZItemSlice() ([]redis.ZItem, error) {
	slice, err := r.cmd.StringSlice()
	if err != nil {
		return nil, err
	}
	val := make([]redis.ZItem, len(slice)/2)
	for i := 0; i < len(val); i++ {
		idx := i * 2
		member := slice[idx]
		score, err := strconv.ParseFloat(slice[idx+1], 64)
		if err != nil {
			return nil, err
		}
		val[i] = redis.ZItem{Score: score, Member: member}
	}
	return val, nil
}

func (r *reply) StringMap() (map[string]string, error) {
	slice, err := r.cmd.StringSlice()
	if err != nil {
		return nil, err
	}
	val := make(map[string]string, len(slice)/2)
	for i := 0; i < len(slice); i += 2 {
		val[slice[i]] = slice[i+1]
	}
	return val, nil
}
