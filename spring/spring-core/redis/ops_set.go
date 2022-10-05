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

// SAdd https://redis.io/commands/sadd
// Command: SADD key member [member ...]
// Integer reply: the number of elements that were added to the set,
// not including all the elements already present in the set.
func (c *Client) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{"SADD", key}
	args = append(args, members...)
	return c.Int(ctx, args...)
}

// SCard https://redis.io/commands/scard
// Command: SCARD key
// Integer reply: the cardinality (number of elements) of the set,
// or 0 if key does not exist.
func (c *Client) SCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{"SCARD", key}
	return c.Int(ctx, args...)
}

// SDiff https://redis.io/commands/sdiff
// Command: SDIFF key [key ...]
// Array reply: list with members of the resulting set.
func (c *Client) SDiff(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{"SDIFF"}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

// SDiffStore https://redis.io/commands/sdiffstore
// Command: SDIFFSTORE destination key [key ...]
// Integer reply: the number of elements in the resulting set.
func (c *Client) SDiffStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{"SDIFFSTORE", destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int(ctx, args...)
}

// SInter https://redis.io/commands/sinter
// Command: SINTER key [key ...]
// Array reply: list with members of the resulting set.
func (c *Client) SInter(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{"SINTER"}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

// SInterStore https://redis.io/commands/sinterstore
// Command: SINTERSTORE destination key [key ...]
// Integer reply: the number of elements in the resulting set.
func (c *Client) SInterStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{"SINTERSTORE", destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int(ctx, args...)
}

// SIsMember https://redis.io/commands/sismember
// Command: SISMEMBER key member
// Integer reply: 1 if the element is a member of the set,
// 0 if the element is not a member of the set, or if key does not exist.
func (c *Client) SIsMember(ctx context.Context, key string, member interface{}) (int64, error) {
	args := []interface{}{"SISMEMBER", key, member}
	return c.Int(ctx, args...)
}

// SMembers https://redis.io/commands/smembers
// Command: SMEMBERS key
// Array reply: all elements of the set.
func (c *Client) SMembers(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{"SMEMBERS", key}
	return c.StringSlice(ctx, args...)
}

// SMIsMember https://redis.io/commands/smismember
// Command: SMISMEMBER key member [member ...]
// Array reply: list representing the membership of the given elements,
// in the same order as they are requested.
func (c *Client) SMIsMember(ctx context.Context, key string, members ...interface{}) ([]int64, error) {
	args := []interface{}{"SMISMEMBER", key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.IntSlice(ctx, args...)
}

// SMove https://redis.io/commands/smove
// Command: SMOVE source destination member
// Integer reply: 1 if the element is moved, 0 if the element
// is not a member of source and no operation was performed.
func (c *Client) SMove(ctx context.Context, source, destination string, member interface{}) (int64, error) {
	args := []interface{}{"SMOVE", source, destination, member}
	return c.Int(ctx, args...)
}

// SPop https://redis.io/commands/spop
// Command: SPOP key [count]
// Bulk string reply: the removed member, or nil when key does not exist.
func (c *Client) SPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{"SPOP", key}
	return c.String(ctx, args...)
}

// SPopN https://redis.io/commands/spop
// Command: SPOP key [count]
// Array reply: the removed members, or an empty array when key does not exist.
func (c *Client) SPopN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{"SPOP", key, count}
	return c.StringSlice(ctx, args...)
}

// SRandMember https://redis.io/commands/srandmember
// Command: SRANDMEMBER key [count]
// Returns a Bulk Reply with the randomly selected element,
// or nil when key does not exist.
func (c *Client) SRandMember(ctx context.Context, key string) (string, error) {
	args := []interface{}{"SRANDMEMBER", key}
	return c.String(ctx, args...)
}

// SRandMemberN https://redis.io/commands/srandmember
// Command: SRANDMEMBER key [count]
// Returns an array of elements, or an empty array when key does not exist.
func (c *Client) SRandMemberN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{"SRANDMEMBER", key, count}
	return c.StringSlice(ctx, args...)
}

// SRem https://redis.io/commands/srem
// Command: SREM key member [member ...]
// Integer reply: the number of members that were removed from the set,
// not including non-existing members.
func (c *Client) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{"SREM", key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.Int(ctx, args...)
}

// SUnion https://redis.io/commands/sunion
// Command: SUNION key [key ...]
// Array reply: list with members of the resulting set.
func (c *Client) SUnion(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{"SUNION"}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

// SUnionStore https://redis.io/commands/sunionstore
// Command: SUNIONSTORE destination key [key ...]
// Integer reply: the number of elements in the resulting set.
func (c *Client) SUnionStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{"SUNIONSTORE", destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int(ctx, args...)
}
