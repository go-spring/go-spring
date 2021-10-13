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
	CommandGetBit   = "GETBIT"
	CommandSetBit   = "SETBIT"
	CommandBitCount = "BITCOUNT"
	CommandBitOp    = "BITOP"
	CommandBitPos   = "BITPOS"
)

type BitmapCommand interface {

	// BitCount https://redis.io/commands/bitcount
	// Count the number of set bits (population counting) in a string.
	// Integer reply: The number of bits set to 1.
	BitCount(ctx context.Context, key string, start, end int64) (int64, error)

	// BitOpAnd https://redis.io/commands/bitop
	// Integer reply: The size of the string stored in the destination key,
	// that is equal to the size of the longest input string.
	BitOpAnd(ctx context.Context, destKey string, keys ...string) (int64, error)

	// BitOpOr https://redis.io/commands/bitop
	// Integer reply: The size of the string stored in the destination key,
	// that is equal to the size of the longest input string.
	BitOpOr(ctx context.Context, destKey string, keys ...string) (int64, error)

	// BitOpXor https://redis.io/commands/bitop
	// Integer reply: The size of the string stored in the destination key,
	// that is equal to the size of the longest input string.
	BitOpXor(ctx context.Context, destKey string, keys ...string) (int64, error)

	// BitOpNot https://redis.io/commands/bitop
	// Integer reply: The size of the string stored in the destination key,
	// that is equal to the size of the longest input string.
	BitOpNot(ctx context.Context, destKey string, key string) (int64, error)

	// BitPos https://redis.io/commands/bitpos
	// Integer reply: The command returns the position of the first bit
	// set to 1 or 0 according to the request.
	BitPos(ctx context.Context, key string, bit int64, start, end int64) (int64, error)

	// GetBit https://redis.io/commands/getbit
	// Integer reply: the bit value stored at offset.
	GetBit(ctx context.Context, key string, offset int64) (int64, error)

	// SetBit https://redis.io/commands/setbit
	// Integer reply: the original bit value stored at offset.
	SetBit(ctx context.Context, key string, offset int64, value int) (int64, error)
}

func (c *BaseClient) BitCount(ctx context.Context, key string, start, end int64) (int64, error) {
	args := []interface{}{CommandBitCount, key, start, end}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) BitOpAnd(ctx context.Context, destKey string, keys ...string) (int64, error) {
	args := []interface{}{CommandBitOp, "AND", destKey}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) BitOpOr(ctx context.Context, destKey string, keys ...string) (int64, error) {
	args := []interface{}{CommandBitOp, "OR", destKey}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) BitOpXor(ctx context.Context, destKey string, keys ...string) (int64, error) {
	args := []interface{}{CommandBitOp, "XOR", destKey}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) BitOpNot(ctx context.Context, destKey string, key string) (int64, error) {
	args := []interface{}{CommandBitOp, "NOT", destKey, key}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) BitPos(ctx context.Context, key string, bit int64, start, end int64) (int64, error) {
	args := []interface{}{CommandBitPos, key, bit, start, end}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) GetBit(ctx context.Context, key string, offset int64) (int64, error) {
	args := []interface{}{CommandGetBit, key, offset}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SetBit(ctx context.Context, key string, offset int64, value int) (int64, error) {
	args := []interface{}{CommandSetBit, key, offset, value}
	return c.Int64(ctx, args...)
}