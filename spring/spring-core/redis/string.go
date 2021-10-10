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
	// Integer reply: the length of the string after the append operation.
	Append(ctx context.Context, key, value string) (int64, error)

	// Decr https://redis.io/commands/decr
	// Integer reply: the value of key after the decrement
	Decr(ctx context.Context, key string) (int64, error)

	// DecrBy https://redis.io/commands/decrby
	// Integer reply: the value of key after the decrement.
	DecrBy(ctx context.Context, key string, decrement int64) (int64, error)

	// Get https://redis.io/commands/get
	// Bulk string reply: the value of key, or nil when key does not exist.
	Get(ctx context.Context, key string) (string, error)

	// GetDel https://redis.io/commands/getdel
	// Bulk string reply: the value of key, nil when key does not exist,
	// or an error if the key's value type isn't a string.
	GetDel(ctx context.Context, key string) (string, error)

	// GetEx https://redis.io/commands/getex
	// Bulk string reply: the value of key, or nil when key does not exist.
	GetEx(ctx context.Context, key string) (string, error)

	// GetRange https://redis.io/commands/getrange
	// Bulk string reply
	GetRange(ctx context.Context, key string, start, end int64) (string, error)

	// GetSet https://redis.io/commands/getset
	// Bulk string reply: the old value stored at key, or nil when key did not exist.
	GetSet(ctx context.Context, key string, value interface{}) (string, error)

	// Incr https://redis.io/commands/incr
	// Integer reply: the value of key after the increment
	Incr(ctx context.Context, key string) (int64, error)

	// IncrBy https://redis.io/commands/incrby
	// Integer reply: the value of key after the increment.
	IncrBy(ctx context.Context, key string, value int64) (int64, error)

	// IncrByFloat https://redis.io/commands/incrbyfloat
	// Bulk string reply: the value of key after the increment.
	IncrByFloat(ctx context.Context, key string, value float64) (float64, error)

	// MGet https://redis.io/commands/mget
	// Array reply: list of values at the specified keys.
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)

	// MSet https://redis.io/commands/mset
	// Simple string reply: always OK since MSET can't fail.
	MSet(ctx context.Context, values ...interface{}) (bool, error)

	// MSetNX https://redis.io/commands/msetnx
	// MSETNX is atomic, so all given keys are set at once
	// Integer reply: 1 if the all the keys were set, 0 if no
	// key was set (at least one key already existed).
	MSetNX(ctx context.Context, values ...interface{}) (bool, error)

	// PSetEX https://redis.io/commands/psetex
	// Simple string reply
	PSetEX(ctx context.Context, key string, value interface{}, expire int64) (bool, error)

	// Set https://redis.io/commands/set
	// Simple string reply: OK if SET was executed correctly.
	Set(ctx context.Context, key string, value interface{}) (bool, error)

	// SetEX https://redis.io/commands/setex
	// Simple string reply
	SetEX(ctx context.Context, key string, value interface{}, expire int64) (bool, error)

	// SetNX https://redis.io/commands/setnx
	// Integer reply: 1 if the key was set, 0 if the key was not set.
	SetNX(ctx context.Context, key string, value interface{}) (bool, error)

	// SetRange https://redis.io/commands/setrange
	// Integer reply: the length of the string after it was modified by the command.
	SetRange(ctx context.Context, key string, offset int64, value string) (int64, error)

	// StrLen https://redis.io/commands/strlen
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

func (c *BaseClient) GetEx(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandGetEx, key}
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

func (c *BaseClient) MSet(ctx context.Context, values ...interface{}) (bool, error) {
	args := []interface{}{CommandMSet}
	for _, value := range values {
		args = append(args, value)
	}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) MSetNX(ctx context.Context, values ...interface{}) (bool, error) {
	args := []interface{}{CommandMSetNX}
	for _, value := range values {
		args = append(args, value)
	}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) PSetEX(ctx context.Context, key string, value interface{}, expire int64) (bool, error) {
	args := []interface{}{CommandPSetEX, key, expire, value}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) Set(ctx context.Context, key string, value interface{}) (bool, error) {
	args := []interface{}{CommandSet, key, value}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) SetEX(ctx context.Context, key string, value interface{}, expire int64) (bool, error) {
	args := []interface{}{CommandSetEX, key, expire, value}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) SetNX(ctx context.Context, key string, value interface{}) (bool, error) {
	args := []interface{}{CommandSetNX, key, value}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) SetRange(ctx context.Context, key string, offset int64, value string) (int64, error) {
	args := []interface{}{CommandSetRange, key, offset, value}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) StrLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandStrLen, key}
	return c.Int64(ctx, args...)
}
