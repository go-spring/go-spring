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
	CommandHDel         = "HDEL"
	CommandHExists      = "HEXISTS"
	CommandHGet         = "HGET"
	CommandHGetAll      = "HGETALL"
	CommandHIncrBy      = "HINCRBY"
	CommandHIncrByFloat = "HINCRBYFLOAT"
	CommandHKeys        = "HKEYS"
	CommandHLen         = "HLEN"
	CommandHMGet        = "HMGET"
	CommandHSet         = "HSET"
	CommandHMSet        = "HMSET"
	CommandHSetNX       = "HSETNX"
	CommandHStrLen      = "HSTRLEN"
	CommandHVals        = "HVALS"
)

type HashCommand interface {

	// HDel https://redis.io/commands/hdel
	// Integer reply: the number of fields that were removed from the
	// hash, not including specified but non existing fields.
	HDel(ctx context.Context, key string, fields ...string) (int64, error)

	// HExists https://redis.io/commands/hexists
	// Integer reply: 1 if the hash contains field, 0 if the hash
	// does not contain field, or key does not exist.
	HExists(ctx context.Context, key, field string) (bool, error)

	// HGet https://redis.io/commands/hget
	// Bulk string reply: the value associated with field, or nil when
	// field is not present in the hash or key does not exist.
	HGet(ctx context.Context, key, field string) (string, error)

	// HGetAll https://redis.io/commands/hgetall
	// Array reply: list of fields and their values stored in the hash,
	// or an empty list when key does not exist.
	HGetAll(ctx context.Context, key string) (map[string]string, error)

	// HIncrBy https://redis.io/commands/hincrby
	// Integer reply: the value at field after the increment operation.
	HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error)

	// HIncrByFloat https://redis.io/commands/hincrbyfloat
	// Bulk string reply: the value of field after the increment.
	HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error)

	// HKeys https://redis.io/commands/hkeys
	// Array reply: list of fields in the hash, or an empty list when key does not exist.
	HKeys(ctx context.Context, key string) ([]string, error)

	// HLen https://redis.io/commands/hlen
	// Integer reply: number of fields in the hash, or 0 when key does not exist.
	HLen(ctx context.Context, key string) (int64, error)

	// HMGet https://redis.io/commands/hmget
	// Array reply: list of values associated with the given fields,
	// in the same order as they are requested.
	HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error)

	// HMSet https://redis.io/commands/hset
	// Simple string reply
	HMSet(ctx context.Context, key string, values ...interface{}) (bool, error)

	// HSet https://redis.io/commands/hmset
	// Integer reply: The number of fields that were added.
	HSet(ctx context.Context, key string, values ...interface{}) (int64, error)

	// HSetNX https://redis.io/commands/hsetnx
	// Integer reply: 1 if field is a new field in the hash and value
	// was set, 0 if field already exists in the hash and no operation was performed.
	HSetNX(ctx context.Context, key, field string, value interface{}) (bool, error)

	// HStrLen https://redis.io/commands/hstrlen
	// Integer reply: the string length of the value associated with field,
	// or zero when field is not present in the hash or key does not exist at all.
	HStrLen(ctx context.Context, key, field string) (int64, error)

	// HVals https://redis.io/commands/hvals
	// Array reply: list of values in the hash, or an empty list when key does not exist.
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
	return c.StringMap(ctx, args...)
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

func (c *BaseClient) HMSet(ctx context.Context, key string, values ...interface{}) (bool, error) {
	args := []interface{}{CommandHMSet, key}
	args = append(args, values...)
	return c.Bool(ctx, args...)
}

func (c *BaseClient) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandHSet, key}
	args = append(args, values...)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) HSetNX(ctx context.Context, key, field string, value interface{}) (bool, error) {
	args := []interface{}{CommandHSetNX, key, field, value}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) HStrLen(ctx context.Context, key, field string) (int64, error) {
	args := []interface{}{CommandHStrLen, key, field}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) HVals(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{CommandHVals, key}
	return c.StringSlice(ctx, args...)
}
