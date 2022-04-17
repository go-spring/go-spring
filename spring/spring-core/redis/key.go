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
	CommandDel       = "DEL"
	CommandDump      = "DUMP"
	CommandExists    = "EXISTS"
	CommandExpire    = "EXPIRE"
	CommandExpireAt  = "EXPIREAT"
	CommandKeys      = "KEYS"
	CommandPersist   = "PERSIST"
	CommandPExpire   = "PEXPIRE"
	CommandPExpireAt = "PEXPIREAT"
	CommandPTTL      = "PTTL"
	CommandRandomKey = "RANDOMKEY"
	CommandRename    = "RENAME"
	CommandRenameNX  = "RENAMENX"
	CommandTouch     = "TOUCH"
	CommandTTL       = "TTL"
	CommandType      = "TYPE"
)

type KeyCommand struct {
	c Redis
}

func NewKeyCommand(c Redis) *KeyCommand {
	return &KeyCommand{c: c}
}

// Del https://redis.io/commands/del
// Command: DEL key [key ...]
// Integer reply: The number of keys that were removed.
func (c *KeyCommand) Del(ctx context.Context, keys ...string) (int64, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int64(ctx, CommandDel, args...)
}

// Dump https://redis.io/commands/dump
// Command: DUMP key
// Bulk string reply: the serialized value.
// If key does not exist a nil bulk reply is returned.
func (c *KeyCommand) Dump(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, CommandDump, args...)
}

// Exists https://redis.io/commands/exists
// Command: EXISTS key [key ...]
// Integer reply: The number of keys existing among the
// ones specified as arguments. Keys mentioned multiple
// times and existing are counted multiple times.
func (c *KeyCommand) Exists(ctx context.Context, keys ...string) (int64, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int64(ctx, CommandExists, args...)
}

// Expire https://redis.io/commands/expire
// Command: EXPIRE key seconds [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyCommand) Expire(ctx context.Context, key string, expire int64, args ...interface{}) (int, error) {
	args = append([]interface{}{key, expire}, args...)
	return c.c.Int(ctx, CommandExpire, args...)
}

// ExpireAt https://redis.io/commands/expireat
// Command: EXPIREAT key timestamp [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyCommand) ExpireAt(ctx context.Context, key string, expireAt int64, args ...interface{}) (int, error) {
	args = append([]interface{}{key, expireAt}, args...)
	return c.c.Int(ctx, CommandExpireAt, args...)
}

// Keys https://redis.io/commands/keys
// Command: KEYS pattern
// Array reply: list of keys matching pattern.
func (c *KeyCommand) Keys(ctx context.Context, pattern string) ([]string, error) {
	args := []interface{}{pattern}
	return c.c.StringSlice(ctx, CommandKeys, args...)
}

// Persist https://redis.io/commands/persist
// Command: PERSIST key
// Integer reply: 1 if the timeout was removed,
// 0 if key does not exist or does not have an associated timeout.
func (c *KeyCommand) Persist(ctx context.Context, key string) (int, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, CommandPersist, args...)
}

// PExpire https://redis.io/commands/pexpire
// Command: PEXPIRE key milliseconds [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyCommand) PExpire(ctx context.Context, key string, expire int64, args ...interface{}) (int, error) {
	args = append([]interface{}{key, expire}, args...)
	return c.c.Int(ctx, CommandPExpire, args...)
}

// PExpireAt https://redis.io/commands/pexpireat
// Command: PEXPIREAT key milliseconds-timestamp [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyCommand) PExpireAt(ctx context.Context, key string, expireAt int64, args ...interface{}) (int, error) {
	args = append([]interface{}{key, expireAt}, args...)
	return c.c.Int(ctx, CommandPExpireAt, args...)
}

// PTTL https://redis.io/commands/pttl
// Command: PTTL key
// Integer reply: TTL in milliseconds, -1 if the key exists
// but has no associated expire, -2 if the key does not exist.
func (c *KeyCommand) PTTL(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int64(ctx, CommandPTTL, args...)
}

// RandomKey https://redis.io/commands/randomkey
// Command: RANDOMKEY
// Bulk string reply: the random key, or nil when the database is empty.
func (c *KeyCommand) RandomKey(ctx context.Context) (string, error) {
	return c.c.String(ctx, CommandRandomKey)
}

// Rename https://redis.io/commands/rename
// Command: RENAME key newkey
// Simple string reply.
func (c *KeyCommand) Rename(ctx context.Context, key, newKey string) (string, error) {
	args := []interface{}{key, newKey}
	return c.c.String(ctx, CommandRename, args...)
}

// RenameNX https://redis.io/commands/renamenx
// Command: RENAMENX key newkey
// Integer reply: 1 if key was renamed to newKey, 0 if newKey already exists.
func (c *KeyCommand) RenameNX(ctx context.Context, key, newKey string) (int, error) {
	args := []interface{}{key, newKey}
	return c.c.Int(ctx, CommandRenameNX, args...)
}

// Touch https://redis.io/commands/touch
// Command: TOUCH key [key ...]
// Integer reply: The number of keys that were touched.
func (c *KeyCommand) Touch(ctx context.Context, keys ...string) (int64, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int64(ctx, CommandTouch, args...)
}

// TTL https://redis.io/commands/ttl
// Command: TTL key
// Integer reply: TTL in seconds, -1 if the key exists
// but has no associated expire, -2 if the key does not exist.
func (c *KeyCommand) TTL(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int64(ctx, CommandTTL, args...)
}

// Type https://redis.io/commands/type
// Command: TYPE key
// Simple string reply: type of key, or none when key does not exist.
func (c *KeyCommand) Type(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, CommandType, args...)
}
