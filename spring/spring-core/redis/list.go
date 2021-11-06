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
	CommandLIndex    = "LINDEX"
	CommandLInsert   = "LINSERT"
	CommandLLen      = "LLEN"
	CommandLMove     = "LMOVE"
	CommandLPop      = "LPOP"
	CommandLPos      = "LPOS"
	CommandLPush     = "LPUSH"
	CommandLPushX    = "LPUSHX"
	CommandLRange    = "LRANGE"
	CommandLRem      = "LREM"
	CommandLSet      = "LSET"
	CommandLTrim     = "LTRIM"
	CommandRPop      = "RPOP"
	CommandRPopLPush = "RPOPLPUSH"
	CommandRPush     = "RPUSH"
	CommandRPushX    = "RPUSHX"
)

type ListCommand interface {

	// LIndex https://redis.io/commands/lindex
	// Command: LINDEX key index
	// Bulk string reply: the requested element, or nil when index is out of range.
	LIndex(ctx context.Context, key string, index int64) (string, error)

	// LInsertBefore https://redis.io/commands/linsert
	// Command: LINSERT key BEFORE|AFTER pivot element
	// Integer reply: the length of the list after the
	// insert operation, or -1 when the value pivot was not found.
	LInsertBefore(ctx context.Context, key string, pivot, value interface{}) (int64, error)

	// LInsertAfter https://redis.io/commands/linsert
	// Command: LINSERT key BEFORE|AFTER pivot element
	// Integer reply: the length of the list after the
	// insert operation, or -1 when the value pivot was not found.
	LInsertAfter(ctx context.Context, key string, pivot, value interface{}) (int64, error)

	// LLen https://redis.io/commands/llen
	// Command: LLEN key
	// Integer reply: the length of the list at key.
	LLen(ctx context.Context, key string) (int64, error)

	// LMove https://redis.io/commands/lmove
	// Command: LMOVE source destination LEFT|RIGHT LEFT|RIGHT
	// Bulk string reply: the element being popped and pushed.
	LMove(ctx context.Context, source, destination, srcPos, destPos string) (string, error)

	// LPop https://redis.io/commands/lpop
	// Command: LPOP key [count]
	// Bulk string reply: the value of the first element, or nil when key does not exist.
	LPop(ctx context.Context, key string) (string, error)

	// LPopN https://redis.io/commands/lpop
	// Command: LPOP key [count]
	// Array reply: list of popped elements, or nil when key does not exist.
	LPopN(ctx context.Context, key string, count int) ([]string, error)

	// LPos https://redis.io/commands/lpos
	// Command: LPOS key element [RANK rank] [COUNT num-matches] [MAXLEN len]
	// The command returns the integer representing the matching element,
	// or nil if there is no match. However, if the COUNT option is given
	// the command returns an array (empty if there are no matches).
	LPos(ctx context.Context, key string, value interface{}, args ...interface{}) (int64, error)

	// LPosN https://redis.io/commands/lpos
	// Command: LPOS key element [RANK rank] [COUNT num-matches] [MAXLEN len]
	// The command returns the integer representing the matching element,
	// or nil if there is no match. However, if the COUNT option is given
	// the command returns an array (empty if there are no matches).
	LPosN(ctx context.Context, key string, value interface{}, count int64, args ...interface{}) ([]int64, error)

	// LPush https://redis.io/commands/lpush
	// Command: LPUSH key element [element ...]
	// Integer reply: the length of the list after the push operations.
	LPush(ctx context.Context, key string, values ...interface{}) (int64, error)

	// LPushX https://redis.io/commands/lpushx
	// Command: LPUSHX key element [element ...]
	// Integer reply: the length of the list after the push operation.
	LPushX(ctx context.Context, key string, values ...interface{}) (int64, error)

	// LRange https://redis.io/commands/lrange
	// Command: LRANGE key start stop
	// Array reply: list of elements in the specified range.
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// LRem https://redis.io/commands/lrem
	// Command: LREM key count element
	// Integer reply: the number of removed elements.
	LRem(ctx context.Context, key string, count int64, value interface{}) (int64, error)

	// LSet https://redis.io/commands/lset
	// Command: LSET key index element
	// Simple string reply
	LSet(ctx context.Context, key string, index int64, value interface{}) (string, error)

	// LTrim https://redis.io/commands/ltrim
	// Command: LTRIM key start stop
	// Simple string reply
	LTrim(ctx context.Context, key string, start, stop int64) (string, error)

	// RPop https://redis.io/commands/rpop
	// Command: RPOP key [count]
	// Bulk string reply: the value of the last element, or nil when key does not exist.
	RPop(ctx context.Context, key string) (string, error)

	// RPopN https://redis.io/commands/rpop
	// Command: RPOP key [count]
	// Array reply: list of popped elements, or nil when key does not exist.
	RPopN(ctx context.Context, key string, count int) ([]string, error)

	// RPopLPush https://redis.io/commands/rpoplpush
	// Command: RPOPLPUSH source destination
	// Bulk string reply: the element being popped and pushed.
	RPopLPush(ctx context.Context, source, destination string) (string, error)

	// RPush https://redis.io/commands/rpush
	// Command: RPUSH key element [element ...]
	// Integer reply: the length of the list after the push operation.
	RPush(ctx context.Context, key string, values ...interface{}) (int64, error)

	// RPushX https://redis.io/commands/rpushx
	// Command: RPUSHX key element [element ...]
	// Integer reply: the length of the list after the push operation.
	RPushX(ctx context.Context, key string, values ...interface{}) (int64, error)
}

func (c *BaseClient) LIndex(ctx context.Context, key string, index int64) (string, error) {
	args := []interface{}{CommandLIndex, key, index}
	return c.String(ctx, args...)
}

func (c *BaseClient) LInsertBefore(ctx context.Context, key string, pivot, value interface{}) (int64, error) {
	args := []interface{}{CommandLInsert, key, "BEFORE", pivot, value}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LInsertAfter(ctx context.Context, key string, pivot, value interface{}) (int64, error) {
	args := []interface{}{CommandLInsert, key, "AFTER", pivot, value}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LLen(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandLLen, key}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LMove(ctx context.Context, source, destination, srcPos, destPos string) (string, error) {
	args := []interface{}{CommandLMove, source, destination, srcPos, destPos}
	return c.String(ctx, args...)
}

func (c *BaseClient) LPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandLPop, key}
	return c.String(ctx, args...)
}

func (c *BaseClient) LPopN(ctx context.Context, key string, count int) ([]string, error) {
	args := []interface{}{CommandLPop, key, count}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) LPos(ctx context.Context, key string, value interface{}, args ...interface{}) (int64, error) {
	args = append([]interface{}{CommandLPos, key, value}, args...)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LPosN(ctx context.Context, key string, value interface{}, count int64, args ...interface{}) ([]int64, error) {
	args = append([]interface{}{CommandLPos, key, value, "COUNT", count}, args...)
	return c.Int64Slice(ctx, args...)
}

func (c *BaseClient) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandLPush, key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LPushX(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandLPushX, key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{CommandLRange, key, start, stop}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) LRem(ctx context.Context, key string, count int64, value interface{}) (int64, error) {
	args := []interface{}{CommandLRem, key, count, value}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LSet(ctx context.Context, key string, index int64, value interface{}) (string, error) {
	args := []interface{}{CommandLSet, key, index, value}
	return c.String(ctx, args...)
}

func (c *BaseClient) LTrim(ctx context.Context, key string, start, stop int64) (string, error) {
	args := []interface{}{CommandLTrim, key, start, stop}
	return c.String(ctx, args...)
}

func (c *BaseClient) RPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandRPop, key}
	return c.String(ctx, args...)
}

func (c *BaseClient) RPopN(ctx context.Context, key string, count int) ([]string, error) {
	args := []interface{}{CommandRPop, key, count}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) RPopLPush(ctx context.Context, source, destination string) (string, error) {
	args := []interface{}{CommandRPopLPush, source, destination}
	return c.String(ctx, args...)
}

func (c *BaseClient) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandRPush, key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) RPushX(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandRPushX, key}
	for _, value := range values {
		args = append(args, value)
	}
	return c.Int64(ctx, args...)
}
