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

// HDel https://redis.io/commands/hdel
// Command: HDEL key field [field ...]
// Integer reply: the number of fields that were removed
// from the hash, not including specified but non-existing fields.
func (c *Client) HDel(ctx context.Context, key string, fields ...string) (int64, error) {
	args := []interface{}{"HDEL", key}
	for _, field := range fields {
		args = append(args, field)
	}
	return c.Int(ctx, args...)
}

// HExists https://redis.io/commands/hexists
// Command: HEXISTS key field
// Integer reply: 1 if the hash contains field,
// 0 if the hash does not contain field, or key does not exist.
func (c *Client) HExists(ctx context.Context, key, field string) (int64, error) {
	args := []interface{}{"HEXISTS", key, field}
	return c.Int(ctx, args...)
}

// HGet https://redis.io/commands/hget
// Command: HGET key field
// Bulk string reply: the value associated with field,
// or nil when field is not present in the hash or key does not exist.
func (c *Client) HGet(ctx context.Context, key string, field string) (string, error) {
	args := []interface{}{"HGET", key, field}
	return c.String(ctx, args...)
}

// HGetAll https://redis.io/commands/hgetall
// Command: HGETALL key
// Array reply: list of fields and their values stored
// in the hash, or an empty list when key does not exist.
func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	args := []interface{}{"HGETALL", key}
	return c.StringMap(ctx, args...)
}

// HIncrBy https://redis.io/commands/hincrby
// Command: HINCRBY key field increment
// Integer reply: the value at field after the increment operation.
func (c *Client) HIncrBy(ctx context.Context, key, field string, incr int64) (int64, error) {
	args := []interface{}{"HINCRBY", key, field, incr}
	return c.Int(ctx, args...)
}

// HIncrByFloat https://redis.io/commands/hincrbyfloat
// Command: HINCRBYFLOAT key field increment
// Bulk string reply: the value of field after the increment.
func (c *Client) HIncrByFloat(ctx context.Context, key, field string, incr float64) (float64, error) {
	args := []interface{}{"HINCRBYFLOAT", key, field, incr}
	return c.Float(ctx, args...)
}

// HKeys https://redis.io/commands/hkeys
// Command: HKEYS key
// Array reply: list of fields in the hash, or an empty list when key does not exist.
func (c *Client) HKeys(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{"HKEYS", key}
	return c.StringSlice(ctx, args...)
}

// HLen https://redis.io/commands/hlen
// Command: HLEN key
// Integer reply: number of fields in the hash, or 0 when key does not exist.
func (c *Client) HLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{"HLEN", key}
	return c.Int(ctx, args...)
}

// HMGet https://redis.io/commands/hmget
// Command: HMGET key field [field ...]
// Array reply: list of values associated with the
// given fields, in the same order as they are requested.
func (c *Client) HMGet(ctx context.Context, key string, fields ...string) ([]interface{}, error) {
	args := []interface{}{"HMGET", key}
	for _, field := range fields {
		args = append(args, field)
	}
	return c.Slice(ctx, args...)
}

// HSet https://redis.io/commands/hset
// Command: HSET key field value [field value ...]
// Integer reply: The number of fields that were added.
func (c *Client) HSet(ctx context.Context, key string, args ...interface{}) (int64, error) {
	args = append([]interface{}{"HSET", key}, args...)
	return c.Int(ctx, args...)
}

// HSetNX https://redis.io/commands/hsetnx
// Command: HSETNX key field value
// Integer reply: 1 if field is a new field in the hash and value was set,
// 0 if field already exists in the hash and no operation was performed.
func (c *Client) HSetNX(ctx context.Context, key, field string, value interface{}) (int64, error) {
	args := []interface{}{"HSETNX", key, field, value}
	return c.Int(ctx, args...)
}

// HStrLen https://redis.io/commands/hstrlen
// Command: HSTRLEN key field
// Integer reply: the string length of the value associated with field,
// or zero when field is not present in the hash or key does not exist at all.
func (c *Client) HStrLen(ctx context.Context, key, field string) (int64, error) {
	args := []interface{}{"HSTRLEN", key, field}
	return c.Int(ctx, args...)
}

// HVals https://redis.io/commands/hvals
// Command: HVALS key
// Array reply: list of values in the hash, or an empty list when key does not exist.
func (c *Client) HVals(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{"HVALS", key}
	return c.StringSlice(ctx, args...)
}
