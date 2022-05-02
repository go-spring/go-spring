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

type StringOperations struct {
	c *Client
}

func NewStringOperations(c *Client) *StringOperations {
	return &StringOperations{c: c}
}

// Append https://redis.io/commands/append
// Command: APPEND key value
// Integer reply: the length of the string after the append operation.
func (c *StringOperations) Append(ctx context.Context, key, value string) (int64, error) {
	args := []interface{}{key, value}
	return c.c.Int(ctx, "APPEND", args...)
}

// Decr https://redis.io/commands/decr
// Command: DECR key
// Integer reply: the value of key after the decrement
func (c *StringOperations) Decr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "DECR", args...)
}

// DecrBy https://redis.io/commands/decrby
// Command: DECRBY key decrement
// Integer reply: the value of key after the decrement.
func (c *StringOperations) DecrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	args := []interface{}{key, decrement}
	return c.c.Int(ctx, "DECRBY", args...)
}

// Get https://redis.io/commands/get
// Command: GET key
// Bulk string reply: the value of key, or nil when key does not exist.
func (c *StringOperations) Get(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, "GET", args...)
}

// GetDel https://redis.io/commands/getdel
// Command: GETDEL key
// Bulk string reply: the value of key, nil when key does not exist,
// or an error if the key's value type isn't a string.
func (c *StringOperations) GetDel(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, "GETDEL", args...)
}

// GetEx https://redis.io/commands/getex
// Command: GETEX key [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|PERSIST]
// Bulk string reply: the value of key, or nil when key does not exist.
func (c *StringOperations) GetEx(ctx context.Context, key string, args ...interface{}) (string, error) {
	args = append([]interface{}{key}, args...)
	return c.c.String(ctx, "GETEX", args...)
}

// GetRange https://redis.io/commands/getrange
// Command: GETRANGE key start end
// Bulk string reply
func (c *StringOperations) GetRange(ctx context.Context, key string, start, end int64) (string, error) {
	args := []interface{}{key, start, end}
	return c.c.String(ctx, "GETRANGE", args...)
}

// GetSet https://redis.io/commands/getset
// Command: GETSET key value
// Bulk string reply: the old value stored at key, or nil when key did not exist.
func (c *StringOperations) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	args := []interface{}{key, value}
	return c.c.String(ctx, "GETSET", args...)
}

// Incr https://redis.io/commands/incr
// Command: INCR key
// Integer reply: the value of key after the increment
func (c *StringOperations) Incr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "INCR", args...)
}

// IncrBy https://redis.io/commands/incrby
// Command: INCRBY key increment
// Integer reply: the value of key after the increment.
func (c *StringOperations) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	args := []interface{}{key, value}
	return c.c.Int(ctx, "INCRBY", args...)
}

// IncrByFloat https://redis.io/commands/incrbyfloat
// Command: INCRBYFLOAT key increment
// Bulk string reply: the value of key after the increment.
func (c *StringOperations) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	args := []interface{}{key, value}
	return c.c.Float(ctx, "INCRBYFLOAT", args...)
}

// MGet https://redis.io/commands/mget
// Command: MGET key [key ...]
// Array reply: list of values at the specified keys.
func (c *StringOperations) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Slice(ctx, "MGET", args...)
}

// MSet https://redis.io/commands/mset
// Command: MSET key value [key value ...]
// Simple string reply: always OK since MSET can't fail.
func (c *StringOperations) MSet(ctx context.Context, args ...interface{}) (string, error) {
	return c.c.String(ctx, "MSET", args...)
}

// MSetNX https://redis.io/commands/msetnx
// Command: MSETNX key value [key value ...]
// MSETNX is atomic, so all given keys are set at once
// Integer reply: 1 if the all the keys were set, 0 if no
// key was set (at least one key already existed).
func (c *StringOperations) MSetNX(ctx context.Context, args ...interface{}) (int64, error) {
	return c.c.Int(ctx, "MSETNX", args...)
}

// PSetEX https://redis.io/commands/psetex
// Command: PSETEX key milliseconds value
// Simple string reply
func (c *StringOperations) PSetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{key, expire, value}
	return c.c.String(ctx, "PSETEX", args...)
}

// Set https://redis.io/commands/set
// Command: SET key value [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|KEEPTTL] [NX|XX] [GET]
// Simple string reply: OK if SET was executed correctly.
func (c *StringOperations) Set(ctx context.Context, key string, value interface{}, args ...interface{}) (string, error) {
	args = append([]interface{}{key, value}, args...)
	return c.c.String(ctx, "SET", args...)
}

// SetEX https://redis.io/commands/setex
// Command: SETEX key seconds value
// Simple string reply
func (c *StringOperations) SetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{key, expire, value}
	return c.c.String(ctx, "SETEX", args...)
}

// SetNX https://redis.io/commands/setnx
// Command: SETNX key value
// Integer reply: 1 if the key was set, 0 if the key was not set.
func (c *StringOperations) SetNX(ctx context.Context, key string, value interface{}) (int64, error) {
	args := []interface{}{key, value}
	return c.c.Int(ctx, "SETNX", args...)
}

// SetRange https://redis.io/commands/setrange
// Command: SETRANGE key offset value
// Integer reply: the length of the string after it was modified by the command.
func (c *StringOperations) SetRange(ctx context.Context, key string, offset int64, value string) (int64, error) {
	args := []interface{}{key, offset, value}
	return c.c.Int(ctx, "SETRANGE", args...)
}

// StrLen https://redis.io/commands/strlen
// Command: STRLEN key
// Integer reply: the length of the string at key, or 0 when key does not exist.
func (c *StringOperations) StrLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "STRLEN", args...)
}
