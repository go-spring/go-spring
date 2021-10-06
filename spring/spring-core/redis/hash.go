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

package redis

import (
	"context"
)

const (
	CommandHDel         = "hdel"
	CommandHExists      = "hexists"
	CommandHGet         = "hget"
	CommandHGetAll      = "hgetall"
	CommandHIncrBy      = "hincrby"
	CommandHIncrByFloat = "hincrbyfloat"
	CommandHKeys        = "hkeys"
	CommandHLen         = "hlen"
	CommandHMGet        = "hmget"
	CommandHSet         = "hset"
	CommandHMSet        = "hmset"
	CommandHSetNX       = "hsetnx"
	CommandHVals        = "hvals"
)

type HashCommand interface {

	// HDel https://redis.io/commands/hdel
	HDel(ctx context.Context, key string, fields ...string) (int64, error)

	// HExists https://redis.io/commands/hexists
	HExists(ctx context.Context, key, field string) (bool, error)

	// HGet https://redis.io/commands/hget
	HGet(ctx context.Context, key, field string) (string, error)

	// HGetAll https://redis.io/commands/hgetall
	HGetAll(ctx context.Context, key string) (map[string]string, error)

	// HIncrBy https://redis.io/commands/hincrby
	HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error)

	// HIncrByFloat https://redis.io/commands/hincrbyfloat
	HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error)

	// HKeys https://redis.io/commands/hkeys
	HKeys(ctx context.Context, key string) ([]string, error)

	// HLen https://redis.io/commands/hlen
	HLen(ctx context.Context, key string) (int64, error)

	// HMGet https://redis.io/commands/hmget
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)

	// HSet https://redis.io/commands/hmset
	HSet(ctx context.Context, key string, values ...interface{}) (int64, error)

	// HMSet https://redis.io/commands/hset
	HMSet(ctx context.Context, key string, values ...interface{}) (bool, error)

	// HSetNX https://redis.io/commands/hsetnx
	HSetNX(ctx context.Context, key, field string, value interface{}) (bool, error)

	// HStrLen https://redis.io/commands/hstrlen

	// HVals https://redis.io/commands/hvals
	HVals(ctx context.Context, key string) ([]string, error)
}

func (c *BaseClient) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	args := []interface{}{CommandHDel, key}
	for _, field := range fields {
		args = append(args, field)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) HExists(ctx context.Context, key, field string) (bool, error) {
	args := []interface{}{CommandHExists, key, field}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) HGet(ctx context.Context, key string, field string) (string, error) {
	args := []interface{}{CommandHGet, key, field}
	return c.String(ctx, args...)
}

func (c *BaseClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := []interface{}{CommandHGetAll, key}
	return c.StringStringMap(ctx, args...)
}

func (c *BaseClient) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	args := []interface{}{CommandHIncrBy, key, field, incr}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error) {
	args := []interface{}{CommandHIncrByFloat, key, field, incr}
	return c.Float64(ctx, args...)
}

func (c *BaseClient) HKeys(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{CommandHKeys, key}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) HLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandHLen, key}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	args := []interface{}{CommandHMGet, key}
	for _, field := range fields {
		args = append(args, field)
	}
	return c.Slice(ctx, args...)
}

func (c *BaseClient) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandHSet, key}
	args = append(args, values...)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) HMSet(ctx context.Context, key string, values ...interface{}) (bool, error) {
	args := []interface{}{CommandHMSet, key}
	args = append(args, values...)
	return c.Bool(ctx, args...)
}

func (c *BaseClient) HSetNX(ctx context.Context, key, field string, value interface{}) (bool, error) {
	args := []interface{}{CommandHSetNX, key, field, value}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) HVals(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{CommandHVals, key}
	return c.StringSlice(ctx, args...)
}
