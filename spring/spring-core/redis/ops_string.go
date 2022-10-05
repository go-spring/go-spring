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

// Append https://redis.io/commands/append
// Command: APPEND key value
// Integer reply: the length of the string after the append operation.
func (c *Client) Append(ctx context.Context, key, value string) (int64, error) {
	args := []interface{}{"APPEND", key, value}
	return c.Int(ctx, args...)
}

// Decr https://redis.io/commands/decr
// Command: DECR key
// Integer reply: the value of key after the decrement
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{"DECR", key}
	return c.Int(ctx, args...)
}

// DecrBy https://redis.io/commands/decrby
// Command: DECRBY key decrement
// Integer reply: the value of key after the decrement.
func (c *Client) DecrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	args := []interface{}{"DECRBY", key, decrement}
	return c.Int(ctx, args...)
}

// Get https://redis.io/commands/get
// Command: GET key
// Bulk string reply: the value of key, or nil when key does not exist.
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	args := []interface{}{"GET", key}
	return c.String(ctx, args...)
}

// GetDel https://redis.io/commands/getdel
// Command: GETDEL key
// Bulk string reply: the value of key, nil when key does not exist,
// or an error if the key's value type isn't a string.
func (c *Client) GetDel(ctx context.Context, key string) (string, error) {
	args := []interface{}{"GETDEL", key}
	return c.String(ctx, args...)
}

// GetEx https://redis.io/commands/getex
// Command: GETEX key [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|PERSIST]
// Bulk string reply: the value of key, or nil when key does not exist.
func (c *Client) GetEx(ctx context.Context, key string, args ...interface{}) (string, error) {
	args = append([]interface{}{"GETEX", key}, args...)
	return c.String(ctx, args...)
}

// GetRange https://redis.io/commands/getrange
// Command: GETRANGE key start end
// Bulk string reply
func (c *Client) GetRange(ctx context.Context, key string, start, end int64) (string, error) {
	args := []interface{}{"GETRANGE", key, start, end}
	return c.String(ctx, args...)
}

// GetSet https://redis.io/commands/getset
// Command: GETSET key value
// Bulk string reply: the old value stored at key, or nil when key did not exist.
func (c *Client) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	args := []interface{}{"GETSET", key, value}
	return c.String(ctx, args...)
}

// Incr https://redis.io/commands/incr
// Command: INCR key
// Integer reply: the value of key after the increment
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{"INCR", key}
	return c.Int(ctx, args...)
}

// IncrBy https://redis.io/commands/incrby
// Command: INCRBY key increment
// Integer reply: the value of key after the increment.
func (c *Client) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	args := []interface{}{"INCRBY", key, value}
	return c.Int(ctx, args...)
}

// IncrByFloat https://redis.io/commands/incrbyfloat
// Command: INCRBYFLOAT key increment
// Bulk string reply: the value of key after the increment.
func (c *Client) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	args := []interface{}{"INCRBYFLOAT", key, value}
	return c.Float(ctx, args...)
}

// MGet https://redis.io/commands/mget
// Command: MGET key [key ...]
// Array reply: list of values at the specified keys.
func (c *Client) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	args := []interface{}{"MGET"}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Slice(ctx, args...)
}

// MSet https://redis.io/commands/mset
// Command: MSET key value [key value ...]
// Simple string reply: always OK since MSET can't fail.
func (c *Client) MSet(ctx context.Context, args ...interface{}) (string, error) {
	args = append([]interface{}{"MSET"}, args...)
	return c.String(ctx, args...)
}

// MSetNX https://redis.io/commands/msetnx
// Command: MSETNX key value [key value ...]
// MSETNX is atomic, so all given keys are set at once
// Integer reply: 1 if the all the keys were set, 0 if no
// key was set (at least one key already existed).
func (c *Client) MSetNX(ctx context.Context, args ...interface{}) (int64, error) {
	args = append([]interface{}{"MSETNX"}, args...)
	return c.Int(ctx, args...)
}

// PSetEX https://redis.io/commands/psetex
// Command: PSETEX key milliseconds value
// Simple string reply
func (c *Client) PSetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{"PSETEX", key, expire, value}
	return c.String(ctx, args...)
}

// Set https://redis.io/commands/set
// Command: SET key value [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|KEEPTTL] [NX|XX] [GET]
// Simple string reply: OK if SET was executed correctly.
func (c *Client) Set(ctx context.Context, key string, value interface{}, args ...interface{}) (string, error) {
	args = append([]interface{}{"SET", key, value}, args...)
	return c.String(ctx, args...)
}

// SetEX https://redis.io/commands/setex
// Command: SETEX key seconds value
// Simple string reply
func (c *Client) SetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{"SETEX", key, expire, value}
	return c.String(ctx, args...)
}

// SetNX https://redis.io/commands/setnx
// Command: SETNX key value
// Integer reply: 1 if the key was set, 0 if the key was not set.
func (c *Client) SetNX(ctx context.Context, key string, value interface{}) (int64, error) {
	args := []interface{}{"SETNX", key, value}
	return c.Int(ctx, args...)
}

// SetRange https://redis.io/commands/setrange
// Command: SETRANGE key offset value
// Integer reply: the length of the string after it was modified by the command.
func (c *Client) SetRange(ctx context.Context, key string, offset int64, value string) (int64, error) {
	args := []interface{}{"SETRANGE", key, offset, value}
	return c.Int(ctx, args...)
}

// StrLen https://redis.io/commands/strlen
// Command: STRLEN key
// Integer reply: the length of the string at key, or 0 when key does not exist.
func (c *Client) StrLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{"STRLEN", key}
	return c.Int(ctx, args...)
}
