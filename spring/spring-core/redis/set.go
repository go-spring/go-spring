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
	// Integer reply: the number of elements that were added to the set,
	// not including all the elements already present in the set.
	SAdd(ctx context.Context, key string, members ...interface{}) (int64, error)

	// SCard https://redis.io/commands/scard
	// Integer reply: the cardinality (number of elements) of the set,
	// or 0 if key does not exist.
	SCard(ctx context.Context, key string) (int64, error)

	// SDiff https://redis.io/commands/sdiff
	// Array reply: list with members of the resulting set.
	SDiff(ctx context.Context, keys ...string) ([]string, error)

	// SDiffStore https://redis.io/commands/sdiffstore
	// Integer reply: the number of elements in the resulting set.
	SDiffStore(ctx context.Context, destination string, keys ...string) (int64, error)

	// SInter https://redis.io/commands/sinter
	// Array reply: list with members of the resulting set.
	SInter(ctx context.Context, keys ...string) ([]string, error)

	// SInterStore https://redis.io/commands/sinterstore
	// Integer reply: the number of elements in the resulting set.
	SInterStore(ctx context.Context, destination string, keys ...string) (int64, error)

	// SIsMember https://redis.io/commands/sismember
	// Integer reply: 1 if the element is a member of the set, 0 if the
	// element is not a member of the set, or if key does not exist.
	SIsMember(ctx context.Context, key string, member interface{}) (bool, error)

	// SMembers https://redis.io/commands/smembers
	// Array reply: all elements of the set.
	SMembers(ctx context.Context, key string) ([]string, error)

	// SMIsMember https://redis.io/commands/smismember
	// Array reply: list representing the membership of the given elements,
	// in the same order as they are requested.
	SMIsMember(ctx context.Context, key string, members ...interface{}) ([]bool, error)

	// SMove https://redis.io/commands/smove
	// Integer reply: 1 if the element is moved, 0 if the element is
	// not a member of source and no operation was performed.
	SMove(ctx context.Context, source, destination string, member interface{}) (bool, error)

	// SPop https://redis.io/commands/spop
	// Bulk string reply: the removed member, or nil when key does not exist.
	SPop(ctx context.Context, key string) (string, error)

	// SPopN https://redis.io/commands/spop
	// Array reply: the removed members, or an empty array when key does not exist.
	SPopN(ctx context.Context, key string, count int64) ([]string, error)

	// SRandMember https://redis.io/commands/srandmember
	// Returns a Bulk Reply with the randomly selected element,
	// or nil when key does not exist.
	SRandMember(ctx context.Context, key string) (string, error)

	// SRandMemberN https://redis.io/commands/srandmember
	// Returns an array of elements, or an empty array when key does not exist.
	SRandMemberN(ctx context.Context, key string, count int64) ([]string, error)

	// SRem https://redis.io/commands/srem
	// Integer reply: the number of members that were removed from the set,
	// not including non existing members.
	SRem(ctx context.Context, key string, members ...interface{}) (int64, error)

	// SUnion https://redis.io/commands/sunion
	// Array reply: list with members of the resulting set.
	SUnion(ctx context.Context, keys ...string) ([]string, error)

	// SUnionStore https://redis.io/commands/sunionstore
	// Integer reply: the number of elements in the resulting set.
	SUnionStore(ctx context.Context, destination string, keys ...string) (int64, error)
}

func (c *BaseClient) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{CommandSAdd, key}
	args = append(args, members...)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandSCard, key}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SDiff(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{CommandSDiff}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) SDiffStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{CommandSDiffStore, destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SInter(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{CommandSInter}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) SInterStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{CommandSInterStore, destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SIsMember(ctx context.Context, key string, member interface{}) (bool, error) {
	args := []interface{}{CommandSIsMember, key, member}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) SMembers(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{CommandSMembers, key}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) SMIsMember(ctx context.Context, key string, members ...interface{}) ([]bool, error) {
	args := []interface{}{CommandSMIsMember, key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.BoolSlice(ctx, args...)
}

func (c *BaseClient) SMove(ctx context.Context, source, destination string, member interface{}) (bool, error) {
	args := []interface{}{CommandSMove, source, destination, member}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) SPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandSPop, key}
	return c.String(ctx, args...)
}

func (c *BaseClient) SPopN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{CommandSPop, key, count}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) SRandMember(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandSRandMember, key}
	return c.String(ctx, args...)
}

func (c *BaseClient) SRandMemberN(ctx context.Context, key string, count int64) ([]string, error) {
	args := []interface{}{CommandSRandMember, key, count}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{CommandSRem, key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SUnion(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{CommandSUnion}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) SUnionStore(ctx context.Context, destination string, keys ...string) (int64, error) {
	args := []interface{}{CommandSUnionStore, destination}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, args...)
}
