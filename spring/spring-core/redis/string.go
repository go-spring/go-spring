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

type StringCommand interface {

	// Append https://redis.io/commands/append
	// Command: APPEND key value
	// Integer reply: the length of the string after the append operation.
	Append(ctx context.Context, key, value string) (int64, error)

	// Decr https://redis.io/commands/decr
	// Command: DECR key
	// Integer reply: the value of key after the decrement
	Decr(ctx context.Context, key string) (int64, error)

	// DecrBy https://redis.io/commands/decrby
	// Command: DECRBY key decrement
	// Integer reply: the value of key after the decrement.
	DecrBy(ctx context.Context, key string, decrement int64) (int64, error)

	// Get https://redis.io/commands/get
	// Command: GET key
	// Bulk string reply: the value of key, or nil when key does not exist.
	Get(ctx context.Context, key string) (string, error)

	// GetDel https://redis.io/commands/getdel
	// Command: GETDEL key
	// Bulk string reply: the value of key, nil when key does not exist,
	// or an error if the key's value type isn't a string.
	GetDel(ctx context.Context, key string) (string, error)

	// GetEx https://redis.io/commands/getex
	// Command: GETEX key [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|PERSIST]
	// Bulk string reply: the value of key, or nil when key does not exist.
	GetEx(ctx context.Context, key string, args ...interface{}) (string, error)

	// GetRange https://redis.io/commands/getrange
	// Command: GETRANGE key start end
	// Bulk string reply
	GetRange(ctx context.Context, key string, start, end int64) (string, error)

	// GetSet https://redis.io/commands/getset
	// Command: GETSET key value
	// Bulk string reply: the old value stored at key, or nil when key did not exist.
	GetSet(ctx context.Context, key string, value interface{}) (string, error)

	// Incr https://redis.io/commands/incr
	// Command: INCR key
	// Integer reply: the value of key after the increment
	Incr(ctx context.Context, key string) (int64, error)

	// IncrBy https://redis.io/commands/incrby
	// Command: INCRBY key increment
	// Integer reply: the value of key after the increment.
	IncrBy(ctx context.Context, key string, value int64) (int64, error)

	// IncrByFloat https://redis.io/commands/incrbyfloat
	// Command: INCRBYFLOAT key increment
	// Bulk string reply: the value of key after the increment.
	IncrByFloat(ctx context.Context, key string, value float64) (float64, error)

	// MGet https://redis.io/commands/mget
	// Command: MGET key [key ...]
	// Array reply: list of values at the specified keys.
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)

	// MSet https://redis.io/commands/mset
	// Command: MSET key value [key value ...]
	// Simple string reply: always OK since MSET can't fail.
	MSet(ctx context.Context, args ...interface{}) (string, error)

	// MSetNX https://redis.io/commands/msetnx
	// Command: MSETNX key value [key value ...]
	// MSETNX is atomic, so all given keys are set at once
	// Integer reply: 1 if the all the keys were set, 0 if no
	// key was set (at least one key already existed).
	MSetNX(ctx context.Context, args ...interface{}) (int, error)

	// PSetEX https://redis.io/commands/psetex
	// Command: PSETEX key milliseconds value
	// Simple string reply
	PSetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error)

	// Set https://redis.io/commands/set
	// Command: SET key value [EX seconds|PX milliseconds|EXAT timestamp|PXAT milliseconds-timestamp|KEEPTTL] [NX|XX] [GET]
	// Simple string reply: OK if SET was executed correctly.
	Set(ctx context.Context, key string, value interface{}, args ...interface{}) (string, error)

	// SetEX https://redis.io/commands/setex
	// Command: SETEX key seconds value
	// Simple string reply
	SetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error)

	// SetNX https://redis.io/commands/setnx
	// Command: SETNX key value
	// Integer reply: 1 if the key was set, 0 if the key was not set.
	SetNX(ctx context.Context, key string, value interface{}) (int, error)

	// SetRange https://redis.io/commands/setrange
	// Command: SETRANGE key offset value
	// Integer reply: the length of the string after it was modified by the command.
	SetRange(ctx context.Context, key string, offset int64, value string) (int64, error)

	// StrLen https://redis.io/commands/strlen
	// Command: STRLEN key
	// Integer reply: the length of the string at key, or 0 when key does not exist.
	StrLen(ctx context.Context, key string) (int64, error)
}

func (c *BaseClient) Append(ctx context.Context, key, value string) (int64, error) {
	args := []interface{}{CommandAppend, key, value}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) Decr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandDecr, key}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) DecrBy(ctx context.Context, key string, decrement int64) (int64, error) {
	args := []interface{}{CommandDecrBy, key, decrement}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) Get(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandGet, key}
	return c.String(ctx, args...)
}

func (c *BaseClient) GetDel(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandGetDel, key}
	return c.String(ctx, args...)
}

func (c *BaseClient) GetEx(ctx context.Context, key string, args ...interface{}) (string, error) {
	args = append([]interface{}{CommandGetEx, key}, args...)
	return c.String(ctx, args...)
}

func (c *BaseClient) GetRange(ctx context.Context, key string, start, end int64) (string, error) {
	args := []interface{}{CommandGetRange, key, start, end}
	return c.String(ctx, args...)
}

func (c *BaseClient) GetSet(ctx context.Context, key string, value interface{}) (string, error) {
	args := []interface{}{CommandGetSet, key, value}
	return c.String(ctx, args...)
}

func (c *BaseClient) Incr(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandIncr, key}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	args := []interface{}{CommandIncrBy, key, value}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) IncrByFloat(ctx context.Context, key string, value float64) (float64, error) {
	args := []interface{}{CommandIncrByFloat, key, value}
	return c.Float64(ctx, args...)
}

func (c *BaseClient) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	args := []interface{}{CommandMGet}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Slice(ctx, args...)
}

func (c *BaseClient) MSet(ctx context.Context, args ...interface{}) (string, error) {
	args = append([]interface{}{CommandMSet}, args...)
	return c.String(ctx, args...)
}

func (c *BaseClient) MSetNX(ctx context.Context, args ...interface{}) (int, error) {
	args = append([]interface{}{CommandMSetNX}, args...)
	return c.Int(ctx, args...)
}

func (c *BaseClient) PSetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{CommandPSetEX, key, expire, value}
	return c.String(ctx, args...)
}

func (c *BaseClient) Set(ctx context.Context, key string, value interface{}, args ...interface{}) (string, error) {
	args = append([]interface{}{CommandSet, key, value}, args...)
	return c.String(ctx, args...)
}

func (c *BaseClient) SetEX(ctx context.Context, key string, value interface{}, expire int64) (string, error) {
	args := []interface{}{CommandSetEX, key, expire, value}
	return c.String(ctx, args...)
}

func (c *BaseClient) SetNX(ctx context.Context, key string, value interface{}) (int, error) {
	args := []interface{}{CommandSetNX, key, value}
	return c.Int(ctx, args...)
}

func (c *BaseClient) SetRange(ctx context.Context, key string, offset int64, value string) (int64, error) {
	args := []interface{}{CommandSetRange, key, offset, value}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) StrLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandStrLen, key}
	return c.Int64(ctx, args...)
}
