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
	CommandAppend      = "APPEND"
	CommandDecr        = "DECR"
	CommandDecrBy      = "DECRBY"
	CommandGet         = "GET"
	CommandGetDel      = "GETDEL"
	CommandGetEx       = "GETEX"
	CommandGetRange    = "GETRANGE"
	CommandGetSet      = "GETSET"
	CommandIncr        = "INCR"
	CommandIncrBy      = "INCRBY"
	CommandIncrByFloat = "INCRBYFLOAT"
	CommandMGet        = "MGET"
	CommandMSet        = "MSET"
	CommandMSetNX      = "MSETNX"
	CommandPSetEX      = "PSETEX"
	CommandSet         = "SET"
	CommandSetEX       = "SETEX"
	CommandSetNX       = "SETNX"
	CommandSetRange    = "SETRANGE"
	CommandStrLen      = "STRLEN"
)

type StringCommand struct {
	c Redis
}

func NewStringCommand(c Redis) *StringCommand {
	return &StringCommand{c: c}
}

// Append https://redis.io/commands/append
// Command: APPEND key value
// Integer reply: the length of the string after the append operation.
func (c *StringCommand) Append(ctx context.Context, key, value string) (int64, error) {
	args := []interface{}{key, value}
	return c.c.Int64(ctx, CommandAppend, args...)
}

// Decr https://redis.io/commands/decr
// Command: DECR key
// Integer reply: the value of key after the decrement
func (c *StringCommand) Decr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int64(ctx, CommandDecr, args...)
}

// DecrBy https://redis.io/commands/decrby
// Command: DECRBY key decrement
// Integer reply: the value of key after the decrement.
func (c *StringCommand) DecrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	args := []interface{}{key, decrement}
	return c.c.Int64(ctx, CommandDecrBy, args...)
}

// Get https://redis.io/commands/get
// Command: GET key
// Bulk string reply: the value of key, or nil when key does not exist.
func (c *StringCommand) Get(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, CommandGet, args...)
}

// GetDel https://redis.io/commands/getdel
// Command: GETDEL key
// Bulk string reply: the value of key, nil when key does not exist,
// or an error if the key's value type isn't a string.
func (c *StringCommand) GetDel(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, CommandGetDel, args...)
}

// GetEx https://redis.io/commands/getex
// Command: GETEX key [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|PERSIST]
// Bulk string reply: the value of key, or nil when key does not exist.
func (c *StringCommand) GetEx(ctx context.Context, key string, args ...interface{}) (string, error) {
	args = append([]interface{}{key}, args...)
	return c.c.String(ctx, CommandGetEx, args...)
}

// GetRange https://redis.io/commands/getrange
// Command: GETRANGE key start end
// Bulk string reply
func (c *StringCommand) GetRange(ctx context.Context, key string, start, end int64) (string, error) {
	args := []interface{}{key, start, end}
	return c.c.String(ctx, CommandGetRange, args...)
}

// GetSet https://redis.io/commands/getset
// Command: GETSET key value
// Bulk string reply: the old value stored at key, or nil when key did not exist.
func (c *StringCommand) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	args := []interface{}{key, value}
	return c.c.String(ctx, CommandGetSet, args...)
}

// Incr https://redis.io/commands/incr
// Command: INCR key
// Integer reply: the value of key after the increment
func (c *StringCommand) Incr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int64(ctx, CommandIncr, args...)
}

// IncrBy https://redis.io/commands/incrby
// Command: INCRBY key increment
// Integer reply: the value of key after the increment.
func (c *StringCommand) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	args := []interface{}{key, value}
	return c.c.Int64(ctx, CommandIncrBy, args...)
}

// IncrByFloat https://redis.io/commands/incrbyfloat
// Command: INCRBYFLOAT key increment
// Bulk string reply: the value of key after the increment.
func (c *StringCommand) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	args := []interface{}{key, value}
	return c.c.Float64(ctx, CommandIncrByFloat, args...)
}

// MGet https://redis.io/commands/mget
// Command: MGET key [key ...]
// Array reply: list of values at the specified keys.
func (c *StringCommand) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Slice(ctx, CommandMGet, args...)
}

// MSet https://redis.io/commands/mset
// Command: MSET key value [key value ...]
// Simple string reply: always OK since MSET can't fail.
func (c *StringCommand) MSet(ctx context.Context, args ...interface{}) (string, error) {
	return c.c.String(ctx, CommandMSet, args...)
}

// MSetNX https://redis.io/commands/msetnx
// Command: MSETNX key value [key value ...]
// MSETNX is atomic, so all given keys are set at once
// Integer reply: 1 if the all the keys were set, 0 if no
// key was set (at least one key already existed).
func (c *StringCommand) MSetNX(ctx context.Context, args ...interface{}) (int, error) {
	return c.c.Int(ctx, CommandMSetNX, args...)
}

// PSetEX https://redis.io/commands/psetex
// Command: PSETEX key milliseconds value
// Simple string reply
func (c *StringCommand) PSetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{key, expire, value}
	return c.c.String(ctx, CommandPSetEX, args...)
}

// Set https://redis.io/commands/set
// Command: SET key value [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|KEEPTTL] [NX|XX] [GET]
// Simple string reply: OK if SET was executed correctly.
func (c *StringCommand) Set(ctx context.Context, key string, value interface{}, args ...interface{}) (string, error) {
	args = append([]interface{}{key, value}, args...)
	return c.c.String(ctx, CommandSet, args...)
}

// SetEX https://redis.io/commands/setex
// Command: SETEX key seconds value
// Simple string reply
func (c *StringCommand) SetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{key, expire, value}
	return c.c.String(ctx, CommandSetEX, args...)
}

// SetNX https://redis.io/commands/setnx
// Command: SETNX key value
// Integer reply: 1 if the key was set, 0 if the key was not set.
func (c *StringCommand) SetNX(ctx context.Context, key string, value interface{}) (int, error) {
	args := []interface{}{key, value}
	return c.c.Int(ctx, CommandSetNX, args...)
}

// SetRange https://redis.io/commands/setrange
// Command: SETRANGE key offset value
// Integer reply: the length of the string after it was modified by the command.
func (c *StringCommand) SetRange(ctx context.Context, key string, offset int64, value string) (int64, error) {
	args := []interface{}{key, offset, value}
	return c.c.Int64(ctx, CommandSetRange, args...)
}

// StrLen https://redis.io/commands/strlen
// Command: STRLEN key
// Integer reply: the length of the string at key, or 0 when key does not exist.
func (c *StringCommand) StrLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int64(ctx, CommandStrLen, args...)
}
