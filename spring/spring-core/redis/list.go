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

type ListOperations struct {
	c *Client
}

func NewListOperations(c *Client) *ListOperations {
	return &ListOperations{c: c}
}

// LIndex https://redis.io/commands/lindex
// Command: LINDEX key index
// Bulk string reply: the requested element, or nil when index is out of range.
func (c *ListOperations) LIndex(ctx context.Context, key string, index int64) (string, error) {
	args := []interface{}{key, index}
	return c.c.String(ctx, "LINDEX", args...)
}

// LInsertBefore https://redis.io/commands/linsert
// Command: LINSERT key BEFORE|AFTER pivot element
// Integer reply: the length of the list after the
// insert operation, or -1 when the value pivot was not found.
func (c *ListOperations) LInsertBefore(ctx context.Context, key string, pivot, value interface{}) (int64, error) {
	args := []interface{}{key, "BEFORE", pivot, value}
	return c.c.Int(ctx, "LINSERT", args...)
}

// LInsertAfter https://redis.io/commands/linsert
// Command: LINSERT key BEFORE|AFTER pivot element
// Integer reply: the length of the list after the
// insert operation, or -1 when the value pivot was not found.
func (c *ListOperations) LInsertAfter(ctx context.Context, key string, pivot, value interface{}) (int64, error) {
	args := []interface{}{key, "AFTER", pivot, value}
	return c.c.Int(ctx, "LINSERT", args...)
}

// LLen https://redis.io/commands/llen
// Command: LLEN key
// Integer reply: the length of the list at key.
func (c *ListOperations) LLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "LLEN", args...)
}

// LMove https://redis.io/commands/lmove
// Command: LMOVE source destination LEFT|RIGHT LEFT|RIGHT
// Bulk string reply: the element being popped and pushed.
func (c *ListOperations) LMove(ctx context.Context, source, destination, srcPos, destPos string) (string, error) {
	args := []interface{}{source, destination, srcPos, destPos}
	return c.c.String(ctx, "LMOVE", args...)
}

// LPop https://redis.io/commands/lpop
// Command: LPOP key [count]
// Bulk string reply: the value of the first element, or nil when key does not exist.
func (c *ListOperations) LPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, "LPOP", args...)
}

// LPopN https://redis.io/commands/lpop
// Command: LPOP key [count]
// Array reply: list of popped elements, or nil when key does not exist.
func (c *ListOperations) LPopN(ctx context.Context, key string, count int) ([]string, error) {
	args := []interface{}{key, count}
	return c.c.StringSlice(ctx, "LPOP", args...)
}

// LPos https://redis.io/commands/lpos
// Command: LPOS key element [RANK rank] [COUNT num-matches] [MAXLEN len]
// The command returns the integer representing the matching element,
// or nil if there is no match. However, if the COUNT option is given
// the command returns an array (empty if there are no matches).
func (c *ListOperations) LPos(ctx context.Context, key string, value interface{}, args ...interface{}) (int64, error) {
	args = append([]interface{}{key, value}, args...)
	return c.c.Int(ctx, "LPOS", args...)
}

// LPosN https://redis.io/commands/lpos
// Command: LPOS key element [RANK rank] [COUNT num-matches] [MAXLEN len]
// The command returns the integer representing the matching element,
// or nil if there is no match. However, if the COUNT option is given
// the command returns an array (empty if there are no matches).
func (c *ListOperations) LPosN(ctx context.Context, key string, value interface{}, count int64, args ...interface{}) ([]int64, error) {
	args = append([]interface{}{key, value, "COUNT", count}, args...)
	return c.c.IntSlice(ctx, "LPOS", args...)
}

// LPush https://redis.io/commands/lpush
// Command: LPUSH key element [element ...]
// Integer reply: the length of the list after the push operations.
func (c *ListOperations) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.c.Int(ctx, "LPUSH", args...)
}

// LPushX https://redis.io/commands/lpushx
// Command: LPUSHX key element [element ...]
// Integer reply: the length of the list after the push operation.
func (c *ListOperations) LPushX(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.c.Int(ctx, "LPUSHX", args...)
}

// LRange https://redis.io/commands/lrange
// Command: LRANGE key start stop
// Array reply: list of elements in the specified range.
func (c *ListOperations) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{key, start, stop}
	return c.c.StringSlice(ctx, "LRANGE", args...)
}

// LRem https://redis.io/commands/lrem
// Command: LREM key count element
// Integer reply: the number of removed elements.
func (c *ListOperations) LRem(ctx context.Context, key string, count int64, value interface{}) (int64, error) {
	args := []interface{}{key, count, value}
	return c.c.Int(ctx, "LREM", args...)
}

// LSet https://redis.io/commands/lset
// Command: LSET key index element
// Simple string reply
func (c *ListOperations) LSet(ctx context.Context, key string, index int64, value interface{}) (string, error) {
	args := []interface{}{key, index, value}
	return c.c.String(ctx, "LSET", args...)
}

// LTrim https://redis.io/commands/ltrim
// Command: LTRIM key start stop
// Simple string reply
func (c *ListOperations) LTrim(ctx context.Context, key string, start, stop int64) (string, error) {
	args := []interface{}{key, start, stop}
	return c.c.String(ctx, "LTRIM", args...)
}

// RPop https://redis.io/commands/rpop
// Command: RPOP key [count]
// Bulk string reply: the value of the last element, or nil when key does not exist.
func (c *ListOperations) RPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, "RPOP", args...)
}

// RPopN https://redis.io/commands/rpop
// Command: RPOP key [count]
// Array reply: list of popped elements, or nil when key does not exist.
func (c *ListOperations) RPopN(ctx context.Context, key string, count int) ([]string, error) {
	args := []interface{}{key, count}
	return c.c.StringSlice(ctx, "RPOP", args...)
}

// RPopLPush https://redis.io/commands/rpoplpush
// Command: RPOPLPUSH source destination
// Bulk string reply: the element being popped and pushed.
func (c *ListOperations) RPopLPush(ctx context.Context, source, destination string) (string, error) {
	args := []interface{}{source, destination}
	return c.c.String(ctx, "RPOPLPUSH", args...)
}

// RPush https://redis.io/commands/rpush
// Command: RPUSH key element [element ...]
// Integer reply: the length of the list after the push operation.
func (c *ListOperations) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.c.Int(ctx, "RPUSH", args...)
}

// RPushX https://redis.io/commands/rpushx
// Command: RPUSHX key element [element ...]
// Integer reply: the length of the list after the push operation.
func (c *ListOperations) RPushX(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.c.Int(ctx, "RPUSHX", args...)
}
