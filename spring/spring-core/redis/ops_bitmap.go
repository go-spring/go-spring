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

// BitCount https://redis.io/commands/bitcount
// Command: BITCOUNT key [start end]
// Integer reply: The number of bits set to 1.
func (c *Client) BitCount(ctx context.Context, key string, args ...interface{}) (int64, error) {
	args = append([]interface{}{"BITCOUNT", key}, args...)
	return c.Int(ctx, args...)
}

// BitOpAnd https://redis.io/commands/bitop
// Command: BITOP AND destkey srckey1 srckey2 srckey3 ... srckeyN
// Integer reply: The size of the string stored in the destination key,
// that is equal to the size of the longest input string.
func (c *Client) BitOpAnd(ctx context.Context, destKey string, keys ...string) (int64, error) {
	args := []interface{}{"BITOP", "AND", destKey}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int(ctx, args...)
}

// BitOpOr https://redis.io/commands/bitop
// Command: BITOP OR destkey srckey1 srckey2 srckey3 ... srckeyN
// Integer reply: The size of the string stored in the destination key,
// that is equal to the size of the longest input string.
func (c *Client) BitOpOr(ctx context.Context, destKey string, keys ...string) (int64, error) {
	args := []interface{}{"BITOP", "OR", destKey}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int(ctx, args...)
}

// BitOpXor https://redis.io/commands/bitop
// Command: BITOP XOR destkey srckey1 srckey2 srckey3 ... srckeyN
// Integer reply: The size of the string stored in the destination key,
// that is equal to the size of the longest input string.
func (c *Client) BitOpXor(ctx context.Context, destKey string, keys ...string) (int64, error) {
	args := []interface{}{"BITOP", "XOR", destKey}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int(ctx, args...)
}

// BitOpNot https://redis.io/commands/bitop
// Command: BITOP NOT destkey srckey
// Integer reply: The size of the string stored in the destination key,
// that is equal to the size of the longest input string.
func (c *Client) BitOpNot(ctx context.Context, destKey string, key string) (int64, error) {
	args := []interface{}{"BITOP", "NOT", destKey, key}
	return c.Int(ctx, args...)
}

// BitPos https://redis.io/commands/bitpos
// Command: BITPOS key bit [start [end]]
// Integer reply: The command returns the position of the first bit
// set to 1 or 0 according to the request.
func (c *Client) BitPos(ctx context.Context, key string, bit int64, args ...interface{}) (int64, error) {
	args = append([]interface{}{"BITPOS", key, bit}, args...)
	return c.Int(ctx, args...)
}

// GetBit https://redis.io/commands/getbit
// Command: GETBIT key offset
// Integer reply: the bit value stored at offset.
func (c *Client) GetBit(ctx context.Context, key string, offset int64) (int64, error) {
	args := []interface{}{"GETBIT", key, offset}
	return c.Int(ctx, args...)
}

// SetBit https://redis.io/commands/setbit
// Command: SETBIT key offset value
// Integer reply: the original bit value stored at offset.
func (c *Client) SetBit(ctx context.Context, key string, offset int64, value int) (int64, error) {
	args := []interface{}{"SETBIT", key, offset, value}
	return c.Int(ctx, args...)
}
