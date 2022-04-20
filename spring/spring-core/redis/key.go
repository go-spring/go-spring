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

type KeyOperations struct {
	c *Client
}

func NewKeyOperations(c *Client) *KeyOperations {
	return &KeyOperations{c: c}
}

// Del https://redis.io/commands/del
// Command: DEL key [key ...]
// Integer reply: The number of keys that were removed.
func (c *KeyOperations) Del(ctx context.Context, keys ...string) (int64, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int(ctx, "DEL", args...)
}

// Dump https://redis.io/commands/dump
// Command: DUMP key
// Bulk string reply: the serialized value.
// If key does not exist a nil bulk reply is returned.
func (c *KeyOperations) Dump(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, "DUMP", args...)
}

// Exists https://redis.io/commands/exists
// Command: EXISTS key [key ...]
// Integer reply: The number of keys existing among the
// ones specified as arguments. Keys mentioned multiple
// times and existing are counted multiple times.
func (c *KeyOperations) Exists(ctx context.Context, keys ...string) (int64, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int(ctx, "EXISTS", args...)
}

// Expire https://redis.io/commands/expire
// Command: EXPIRE key seconds [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyOperations) Expire(ctx context.Context, key string, expire int64, args ...interface{}) (int64, error) {
	args = append([]interface{}{key, expire}, args...)
	return c.c.Int(ctx, "EXPIRE", args...)
}

// ExpireAt https://redis.io/commands/expireat
// Command: EXPIREAT key timestamp [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyOperations) ExpireAt(ctx context.Context, key string, expireAt int64, args ...interface{}) (int64, error) {
	args = append([]interface{}{key, expireAt}, args...)
	return c.c.Int(ctx, "EXPIREAT", args...)
}

// Keys https://redis.io/commands/keys
// Command: KEYS pattern
// Array reply: list of keys matching pattern.
func (c *KeyOperations) Keys(ctx context.Context, pattern string) ([]string, error) {
	args := []interface{}{pattern}
	return c.c.StringSlice(ctx, "KEYS", args...)
}

// Persist https://redis.io/commands/persist
// Command: PERSIST key
// Integer reply: 1 if the timeout was removed,
// 0 if key does not exist or does not have an associated timeout.
func (c *KeyOperations) Persist(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "PERSIST", args...)
}

// PExpire https://redis.io/commands/pexpire
// Command: PEXPIRE key milliseconds [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyOperations) PExpire(ctx context.Context, key string, expire int64, args ...interface{}) (int64, error) {
	args = append([]interface{}{key, expire}, args...)
	return c.c.Int(ctx, "PEXPIRE", args...)
}

// PExpireAt https://redis.io/commands/pexpireat
// Command: PEXPIREAT key milliseconds-timestamp [NX|XX|GT|LT]
// Integer reply: 1 if the timeout was set, 0 if the timeout was not set.
func (c *KeyOperations) PExpireAt(ctx context.Context, key string, expireAt int64, args ...interface{}) (int64, error) {
	args = append([]interface{}{key, expireAt}, args...)
	return c.c.Int(ctx, "PEXPIREAT", args...)
}

// PTTL https://redis.io/commands/pttl
// Command: PTTL key
// Integer reply: TTL in milliseconds, -1 if the key exists
// but has no associated expire, -2 if the key does not exist.
func (c *KeyOperations) PTTL(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "PTTL", args...)
}

// RandomKey https://redis.io/commands/randomkey
// Command: RANDOMKEY
// Bulk string reply: the random key, or nil when the database is empty.
func (c *KeyOperations) RandomKey(ctx context.Context) (string, error) {
	return c.c.String(ctx, "RANDOMKEY")
}

// Rename https://redis.io/commands/rename
// Command: RENAME key newkey
// Simple string reply.
func (c *KeyOperations) Rename(ctx context.Context, key, newKey string) (string, error) {
	args := []interface{}{key, newKey}
	return c.c.String(ctx, "RENAME", args...)
}

// RenameNX https://redis.io/commands/renamenx
// Command: RENAMENX key newkey
// Integer reply: 1 if key was renamed to newKey, 0 if newKey already exists.
func (c *KeyOperations) RenameNX(ctx context.Context, key, newKey string) (int64, error) {
	args := []interface{}{key, newKey}
	return c.c.Int(ctx, "RENAMENX", args...)
}

// Touch https://redis.io/commands/touch
// Command: TOUCH key [key ...]
// Integer reply: The number of keys that were touched.
func (c *KeyOperations) Touch(ctx context.Context, keys ...string) (int64, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int(ctx, "TOUCH", args...)
}

// TTL https://redis.io/commands/ttl
// Command: TTL key
// Integer reply: TTL in seconds, -1 if the key exists
// but has no associated expire, -2 if the key does not exist.
func (c *KeyOperations) TTL(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "TTL", args...)
}

// Type https://redis.io/commands/type
// Command: TYPE key
// Simple string reply: type of key, or none when key does not exist.
func (c *KeyOperations) Type(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, "TYPE", args...)
}
