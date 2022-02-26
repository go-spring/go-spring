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
	CommandSAdd        = "SADD"
	CommandSCard       = "SCARD"
	CommandSDiff       = "SDIFF"
	CommandSDiffStore  = "SDIFFSTORE"
	CommandSInter      = "SINTER"
	CommandSInterStore = "SINTERSTORE"
	CommandSIsMember   = "SISMEMBER"
	CommandSMIsMember  = "SMISMEMBER"
	CommandSMembers    = "SMEMBERS"
	CommandSMove       = "SMOVE"
	CommandSPop        = "SPOP"
	CommandSRandMember = "SRANDMEMBER"
	CommandSRem        = "SREM"
	CommandSUnion      = "SUNION"
	CommandSUnionStore = "SUNIONSTORE"
)

type SetCommand interface {

	// SAdd https://redis.io/commands/sadd
	// Command: SADD key member [member ...]
	// Integer reply: the number of elements that were added to the set,
	// not including all the elements already present in the set.
	SAdd(ctx context.Context, key string, members ...interface{}) (int64, error)

	// SCard https://redis.io/commands/scard
	// Command: SCARD key
	// Integer reply: the cardinality (number of elements) of the set,
	// or 0 if key does not exist.
	SCard(ctx context.Context, key string) (int64, error)

	// SDiff https://redis.io/commands/sdiff
	// Command: SDIFF key [key ...]
	// Array reply: list with members of the resulting set.
	SDiff(ctx context.Context, keys ...string) ([]string, error)

	// SDiffStore https://redis.io/commands/sdiffstore
	// Command: SDIFFSTORE destination key [key ...]
	// Integer reply: the number of elements in the resulting set.
	SDiffStore(ctx context.Context, destination string, keys ...string) (int64, error)

	// SInter https://redis.io/commands/sinter
	// Command: SINTER key [key ...]
	// Array reply: list with members of the resulting set.
	SInter(ctx context.Context, keys ...string) ([]string, error)

	// SInterStore https://redis.io/commands/sinterstore
	// Command: SINTERSTORE destination key [key ...]
	// Integer reply: the number of elements in the resulting set.
	SInterStore(ctx context.Context, destination string, keys ...string) (int64, error)

	// SIsMember https://redis.io/commands/sismember
	// Command: SISMEMBER key member
	// Integer reply: 1 if the element is a member of the set,
	// 0 if the element is not a member of the set, or if key does not exist.
	SIsMember(ctx context.Context, key string, member interface{}) (int, error)

	// SMembers https://redis.io/commands/smembers
	// Command: SMEMBERS key
	// Array reply: all elements of the set.
	SMembers(ctx context.Context, key string) ([]string, error)

	// SMIsMember https://redis.io/commands/smismember
	// Command: SMISMEMBER key member [member ...]
	// Array reply: list representing the membership of the given elements,
	// in the same order as they are requested.
	SMIsMember(ctx context.Context, key string, members ...interface{}) ([]int64, error)

	// SMove https://redis.io/commands/smove
	// Command: SMOVE source destination member
	// Integer reply: 1 if the element is moved, 0 if the element
	// is not a member of source and no operation was performed.
	SMove(ctx context.Context, source, destination string, member interface{}) (int, error)

	// SPop https://redis.io/commands/spop
	// Command: SPOP key [count]
	// Bulk string reply: the removed member, or nil when key does not exist.
	SPop(ctx context.Context, key string) (string, error)

	// SPopN https://redis.io/commands/spop
	// Command: SPOP key [count]
	// Array reply: the removed members, or an empty array when key does not exist.
	SPopN(ctx context.Context, key string, count int64) ([]string, error)

	// SRandMember https://redis.io/commands/srandmember
	// Command: SRANDMEMBER key [count]
	// Returns a Bulk Reply with the randomly selected element,
	// or nil when key does not exist.
	SRandMember(ctx context.Context, key string) (string, error)

	// SRandMemberN https://redis.io/commands/srandmember
	// Command: SRANDMEMBER key [count]
	// Returns an array of elements, or an empty array when key does not exist.
	SRandMemberN(ctx context.Context, key string, count int64) ([]string, error)

	// SRem https://redis.io/commands/srem
	// Command: SREM key member [member ...]
	// Integer reply: the number of members that were removed from the set,
	// not including non existing members.
	SRem(ctx context.Context, key string, members ...interface{}) (int64, error)

	// SUnion https://redis.io/commands/sunion
	// Command: SUNION key [key ...]
	// Array reply: list with members of the resulting set.
	SUnion(ctx context.Context, keys ...string) ([]string, error)

	// SUnionStore https://redis.io/commands/sunionstore
	// Command: SUNIONSTORE destination key [key ...]
	// Integer reply: the number of elements in the resulting set.
	SUnionStore(ctx context.Context, destination string, keys ...string) (int64, error)
}

func (c *client) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{key}
	args = append(args, members...)
	return c.Int64(ctx, CommandSAdd, args...)
}

func (c *client) SCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.Int64(ctx, CommandSCard, args...)
}

func (c *client) SDiff(ctx context.Context, keys ...string) ([]string, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, CommandSDiff, args...)
}

func (c *client) SDiffStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, CommandSDiffStore, args...)
}

func (c *client) SInter(ctx context.Context, keys ...string) ([]string, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, CommandSInter, args...)
}

func (c *client) SInterStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, CommandSInterStore, args...)
}

func (c *client) SIsMember(ctx context.Context, key string, member interface{}) (int, error) {
	args := []interface{}{key, member}
	return c.Int(ctx, CommandSIsMember, args...)
}

func (c *client) SMembers(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{key}
	return c.StringSlice(ctx, CommandSMembers, args...)
}

func (c *client) SMIsMember(ctx context.Context, key string, members ...interface{}) ([]int64, error) {
	args := []interface{}{key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.Int64Slice(ctx, CommandSMIsMember, args...)
}

func (c *client) SMove(ctx context.Context, source, destination string, member interface{}) (int, error) {
	args := []interface{}{source, destination, member}
	return c.Int(ctx, CommandSMove, args...)
}

func (c *client) SPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.String(ctx, CommandSPop, args...)
}

func (c *client) SPopN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{key, count}
	return c.StringSlice(ctx, CommandSPop, args...)
}

func (c *client) SRandMember(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.String(ctx, CommandSRandMember, args...)
}

func (c *client) SRandMemberN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{key, count}
	return c.StringSlice(ctx, CommandSRandMember, args...)
}

func (c *client) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.Int64(ctx, CommandSRem, args...)
}

func (c *client) SUnion(ctx context.Context, keys ...string) ([]string, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, CommandSUnion, args...)
}

func (c *client) SUnionStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, CommandSUnionStore, args...)
}
