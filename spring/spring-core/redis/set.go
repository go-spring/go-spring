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

type SetCommand struct {
	c Redis
}

func NewSetCommand(c Redis) *SetCommand {
	return &SetCommand{c: c}
}

// SAdd https://redis.io/commands/sadd
// Command: SADD key member [member ...]
// Integer reply: the number of elements that were added to the set,
// not including all the elements already present in the set.
func (c *SetCommand) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{key}
	args = append(args, members...)
	return c.c.Int64(ctx, CommandSAdd, args...)
}

// SCard https://redis.io/commands/scard
// Command: SCARD key
// Integer reply: the cardinality (number of elements) of the set,
// or 0 if key does not exist.
func (c *SetCommand) SCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int64(ctx, CommandSCard, args...)
}

// SDiff https://redis.io/commands/sdiff
// Command: SDIFF key [key ...]
// Array reply: list with members of the resulting set.
func (c *SetCommand) SDiff(ctx context.Context, keys ...string) ([]string, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.StringSlice(ctx, CommandSDiff, args...)
}

// SDiffStore https://redis.io/commands/sdiffstore
// Command: SDIFFSTORE destination key [key ...]
// Integer reply: the number of elements in the resulting set.
func (c *SetCommand) SDiffStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int64(ctx, CommandSDiffStore, args...)
}

// SInter https://redis.io/commands/sinter
// Command: SINTER key [key ...]
// Array reply: list with members of the resulting set.
func (c *SetCommand) SInter(ctx context.Context, keys ...string) ([]string, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.StringSlice(ctx, CommandSInter, args...)
}

// SInterStore https://redis.io/commands/sinterstore
// Command: SINTERSTORE destination key [key ...]
// Integer reply: the number of elements in the resulting set.
func (c *SetCommand) SInterStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int64(ctx, CommandSInterStore, args...)
}

// SIsMember https://redis.io/commands/sismember
// Command: SISMEMBER key member
// Integer reply: 1 if the element is a member of the set,
// 0 if the element is not a member of the set, or if key does not exist.
func (c *SetCommand) SIsMember(ctx context.Context, key string, member interface{}) (int, error) {
	args := []interface{}{key, member}
	return c.c.Int(ctx, CommandSIsMember, args...)
}

// SMembers https://redis.io/commands/smembers
// Command: SMEMBERS key
// Array reply: all elements of the set.
func (c *SetCommand) SMembers(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{key}
	return c.c.StringSlice(ctx, CommandSMembers, args...)
}

// SMIsMember https://redis.io/commands/smismember
// Command: SMISMEMBER key member [member ...]
// Array reply: list representing the membership of the given elements,
// in the same order as they are requested.
func (c *SetCommand) SMIsMember(ctx context.Context, key string, members ...interface{}) ([]int64, error) {
	args := []interface{}{key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.c.Int64Slice(ctx, CommandSMIsMember, args...)
}

// SMove https://redis.io/commands/smove
// Command: SMOVE source destination member
// Integer reply: 1 if the element is moved, 0 if the element
// is not a member of source and no operation was performed.
func (c *SetCommand) SMove(ctx context.Context, source, destination string, member interface{}) (int, error) {
	args := []interface{}{source, destination, member}
	return c.c.Int(ctx, CommandSMove, args...)
}

// SPop https://redis.io/commands/spop
// Command: SPOP key [count]
// Bulk string reply: the removed member, or nil when key does not exist.
func (c *SetCommand) SPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, CommandSPop, args...)
}

// SPopN https://redis.io/commands/spop
// Command: SPOP key [count]
// Array reply: the removed members, or an empty array when key does not exist.
func (c *SetCommand) SPopN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{key, count}
	return c.c.StringSlice(ctx, CommandSPop, args...)
}

// SRandMember https://redis.io/commands/srandmember
// Command: SRANDMEMBER key [count]
// Returns a Bulk Reply with the randomly selected element,
// or nil when key does not exist.
func (c *SetCommand) SRandMember(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, CommandSRandMember, args...)
}

// SRandMemberN https://redis.io/commands/srandmember
// Command: SRANDMEMBER key [count]
// Returns an array of elements, or an empty array when key does not exist.
func (c *SetCommand) SRandMemberN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{key, count}
	return c.c.StringSlice(ctx, CommandSRandMember, args...)
}

// SRem https://redis.io/commands/srem
// Command: SREM key member [member ...]
// Integer reply: the number of members that were removed from the set,
// not including non existing members.
func (c *SetCommand) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.c.Int64(ctx, CommandSRem, args...)
}

// SUnion https://redis.io/commands/sunion
// Command: SUNION key [key ...]
// Array reply: list with members of the resulting set.
func (c *SetCommand) SUnion(ctx context.Context, keys ...string) ([]string, error) {
	var args []interface{}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.StringSlice(ctx, CommandSUnion, args...)
}

// SUnionStore https://redis.io/commands/sunionstore
// Command: SUNIONSTORE destination key [key ...]
// Integer reply: the number of elements in the resulting set.
func (c *SetCommand) SUnionStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.Int64(ctx, CommandSUnionStore, args...)
}
