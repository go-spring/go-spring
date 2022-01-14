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
	CommandZAdd             = "ZADD"
	CommandZCard            = "ZCARD"
	CommandZCount           = "ZCOUNT"
	CommandZDiff            = "ZDIFF"
	CommandZIncrBy          = "ZINCRBY"
	CommandZInter           = "ZINTER"
	CommandZLexCount        = "ZLEXCOUNT"
	CommandZMScore          = "ZMSCORE"
	CommandZPopMax          = "ZPOPMAX"
	CommandZPopMin          = "ZPOPMIN"
	CommandZRandMember      = "ZRANDMEMBER"
	CommandZRange           = "ZRANGE"
	CommandZRangeByLex      = "ZRANGEBYLEX"
	CommandZRangeByScore    = "ZRANGEBYSCORE"
	CommandZRank            = "ZRANK"
	CommandZRem             = "ZREM"
	CommandZRemRangeByLex   = "ZREMRANGEBYLEX"
	CommandZRemRangeByRank  = "ZREMRANGEBYRANK"
	CommandZRemRangeByScore = "ZREMRANGEBYSCORE"
	CommandZRevRange        = "ZREVRANGE"
	CommandZRevRangeByLex   = "ZREVRANGEBYLEX"
	CommandZRevRangeByScore = "ZREVRANGEBYSCORE"
	CommandZRevRank         = "ZREVRANK"
	CommandZScore           = "ZSCORE"
	CommandZUnion           = "ZUNION"
	CommandZUnionStore      = "ZUNIONSTORE"
)

type ZItem struct {
	Member interface{}
	Score  float64
}

type ZSetCommand interface {

	// ZAdd https://redis.io/commands/zadd
	// Command: ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
	// Integer reply, the number of elements added to the
	// sorted set (excluding score updates).
	ZAdd(ctx context.Context, key string, args ...interface{}) (int64, error)

	// ZCard https://redis.io/commands/zcard
	// Command: ZCARD key
	// Integer reply: the cardinality (number of elements)
	// of the sorted set, or 0 if key does not exist.
	ZCard(ctx context.Context, key string) (int64, error)

	// ZCount https://redis.io/commands/zcount
	// Command: ZCOUNT key min max
	// Integer reply: the number of elements in the specified score range.
	ZCount(ctx context.Context, key, min, max string) (int64, error)

	// ZDiff https://redis.io/commands/zdiff
	// Command: ZDIFF numkeys key [key ...] [WITHSCORES]
	// Array reply: the result of the difference.
	ZDiff(ctx context.Context, keys ...string) ([]string, error)

	// ZDiffWithScores https://redis.io/commands/zdiff
	// Command: ZDIFF numkeys key [key ...] [WITHSCORES]
	// Array reply: the result of the difference.
	ZDiffWithScores(ctx context.Context, keys ...string) ([]ZItem, error)

	// ZIncrBy https://redis.io/commands/zincrby
	// Command: ZINCRBY key increment member
	// Bulk string reply: the new score of member
	// (a double precision floating point number), represented as string.
	ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error)

	// ZInter https://redis.io/commands/zinter
	// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
	// Array reply: the result of intersection.
	ZInter(ctx context.Context, args ...interface{}) ([]string, error)

	// ZInterWithScores https://redis.io/commands/zinter
	// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
	// Array reply: the result of intersection.
	ZInterWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error)

	// ZLexCount https://redis.io/commands/zlexcount
	// Command: ZLEXCOUNT key min max
	// Integer reply: the number of elements in the specified score range.
	ZLexCount(ctx context.Context, key, min, max string) (int64, error)

	// ZMScore https://redis.io/commands/zmscore
	// Command: ZMSCORE key member [member ...]
	// Array reply: list of scores or nil associated with the specified member
	// values (a double precision floating point number), represented as strings.
	ZMScore(ctx context.Context, key string, members ...string) ([]float64, error)

	// ZPopMax https://redis.io/commands/zpopmax
	// Command: ZPOPMAX key [count]
	// Array reply: list of popped elements and scores.
	ZPopMax(ctx context.Context, key string) ([]ZItem, error)

	// ZPopMaxN https://redis.io/commands/zpopmax
	// Command: ZPOPMAX key [count]
	// Array reply: list of popped elements and scores.
	ZPopMaxN(ctx context.Context, key string, count int64) ([]ZItem, error)

	// ZPopMin https://redis.io/commands/zpopmin
	// Command: ZPOPMIN key [count]
	// Array reply: list of popped elements and scores.
	ZPopMin(ctx context.Context, key string) ([]ZItem, error)

	// ZPopMinN https://redis.io/commands/zpopmin
	// Command: ZPOPMIN key [count]
	// Array reply: list of popped elements and scores.
	ZPopMinN(ctx context.Context, key string, count int64) ([]ZItem, error)

	// ZRandMember https://redis.io/commands/zrandmember
	// Command: ZRANDMEMBER key [count [WITHSCORES]]
	// Bulk Reply with the randomly selected element, or nil when key does not exist.
	ZRandMember(ctx context.Context, key string) (string, error)

	// ZRandMemberN https://redis.io/commands/zrandmember
	// Command: ZRANDMEMBER key [count [WITHSCORES]]
	// Bulk Reply with the randomly selected element, or nil when key does not exist.
	ZRandMemberN(ctx context.Context, key string, count int) ([]string, error)

	// ZRandMemberWithScores https://redis.io/commands/zrandmember
	// Command: ZRANDMEMBER key [count [WITHSCORES]]
	// Bulk Reply with the randomly selected element, or nil when key does not exist.
	ZRandMemberWithScores(ctx context.Context, key string, count int) ([]ZItem, error)

	// ZRange https://redis.io/commands/zrange
	// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
	// Array reply: list of elements in the specified range.
	ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]string, error)

	// ZRangeWithScores https://redis.io/commands/zrange
	// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
	// Array reply: list of elements in the specified range.
	ZRangeWithScores(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]ZItem, error)

	// ZRangeByLex https://redis.io/commands/zrangebylex
	// Command: ZRANGEBYLEX key min max [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error)

	// ZRangeByScore https://redis.io/commands/zrangebyscore
	// Command: ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error)

	// ZRank https://redis.io/commands/zrank
	// Command: ZRANK key member
	// If member exists in the sorted set, Integer reply: the rank of member.
	// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
	ZRank(ctx context.Context, key, member string) (int64, error)

	// ZRem https://redis.io/commands/zrem
	// Command: ZREM key member [member ...]
	// Integer reply, The number of members removed from the sorted set, not including non existing members.
	ZRem(ctx context.Context, key string, members ...interface{}) (int64, error)

	// ZRemRangeByLex https://redis.io/commands/zremrangebylex
	// Command: ZREMRANGEBYLEX key min max
	// Integer reply: the number of elements removed.
	ZRemRangeByLex(ctx context.Context, key, min, max string) (int64, error)

	// ZRemRangeByRank https://redis.io/commands/zremrangebyrank
	// Command: ZREMRANGEBYRANK key start stop
	// Integer reply: the number of elements removed.
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) (int64, error)

	// ZRemRangeByScore https://redis.io/commands/zremrangebyscore
	// Command: ZREMRANGEBYSCORE key min max
	// Integer reply: the number of elements removed.
	ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error)

	// ZRevRange https://redis.io/commands/zrevrange
	// Command: ZREVRANGE key start stop [WITHSCORES]
	// Array reply: list of elements in the specified range.
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// ZRevRangeWithScores https://redis.io/commands/zrevrange
	// Command: ZREVRANGE key start stop [WITHSCORES]
	// Array reply: list of elements in the specified range.
	ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]string, error)

	// ZRevRangeByLex https://redis.io/commands/zrevrangebylex
	// Command: ZREVRANGEBYLEX key max min [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRevRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error)

	// ZRevRangeByScore https://redis.io/commands/zrevrangebyscore
	// Command: ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRevRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error)

	// ZRevRank https://redis.io/commands/zrevrank
	// Command: ZREVRANK key member
	// If member exists in the sorted set, Integer reply: the rank of member.
	// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
	ZRevRank(ctx context.Context, key, member string) (int64, error)

	// ZScore https://redis.io/commands/zscore
	// Command: ZSCORE key member
	// Bulk string reply: the score of member (a double precision floating point number), represented as string.
	ZScore(ctx context.Context, key, member string) (float64, error)

	// ZUnion https://redis.io/commands/zunion
	// Command: ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
	// Array reply: the result of union.
	ZUnion(ctx context.Context, args ...interface{}) ([]string, error)

	// ZUnionWithScores https://redis.io/commands/zunion
	// Command: ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
	// Array reply: the result of union.
	ZUnionWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error)

	// ZUnionStore https://redis.io/commands/zunionstore
	// Command: ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
	// Integer reply: the number of elements in the resulting sorted set at destination.
	ZUnionStore(ctx context.Context, dest string, args ...interface{}) (int64, error)
}

func (c *client) ZAdd(ctx context.Context, key string, args ...interface{}) (int64, error) {
	args = append([]interface{}{CommandZAdd, key}, args...)
	return c.Int64(ctx, args...)
}

func (c *client) ZCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandZCard, key}
	return c.Int64(ctx, args...)
}

func (c *client) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{CommandZCount, key, min, max}
	return c.Int64(ctx, args...)
}

func (c *client) ZDiff(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{CommandZDiff, len(keys)}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

func (c *client) ZDiffWithScores(ctx context.Context, keys ...string) ([]ZItem, error) {
	args := []interface{}{CommandZDiff, len(keys)}
	for _, key := range keys {
		args = append(args, key)
	}
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	args := []interface{}{CommandZIncrBy, key, increment, member}
	return c.Float64(ctx, args...)
}

func (c *client) ZInter(ctx context.Context, args ...interface{}) ([]string, error) {
	args = append([]interface{}{CommandZInter}, args...)
	return c.StringSlice(ctx, args...)
}

func (c *client) ZInterWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	args = append([]interface{}{CommandZInter}, args...)
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZLexCount(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{CommandZLexCount, key, min, max}
	return c.Int64(ctx, args...)
}

func (c *client) ZMScore(ctx context.Context, key string, members ...string) ([]float64, error) {
	args := []interface{}{CommandZMScore, key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.Float64Slice(ctx, args...)
}

func (c *client) ZPopMax(ctx context.Context, key string) ([]ZItem, error) {
	args := []interface{}{CommandZPopMax, key}
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZPopMaxN(ctx context.Context, key string, count int64) ([]ZItem, error) {
	args := []interface{}{CommandZPopMax, key, count}
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZPopMin(ctx context.Context, key string) ([]ZItem, error) {
	args := []interface{}{CommandZPopMin, key}
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZPopMinN(ctx context.Context, key string, count int64) ([]ZItem, error) {
	args := []interface{}{CommandZPopMin, key, count}
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZRandMember(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandZRandMember, key}
	return c.String(ctx, args...)
}

func (c *client) ZRandMemberN(ctx context.Context, key string, count int) ([]string, error) {
	args := []interface{}{CommandZRandMember, key, count}
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRandMemberWithScores(ctx context.Context, key string, count int) ([]ZItem, error) {
	args := []interface{}{CommandZRandMember, key, count, "WITHSCORES"}
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]string, error) {
	args = append([]interface{}{CommandZRange, key, start, stop}, args...)
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRangeWithScores(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]ZItem, error) {
	args = append([]interface{}{CommandZRange, key, start, stop}, args...)
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{CommandZRangeByLex, key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{CommandZRangeByScore, key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRank(ctx context.Context, key, member string) (int64, error) {
	args := []interface{}{CommandZRank, key, member}
	return c.Int64(ctx, args...)
}

func (c *client) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{CommandZRem, key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.Int64(ctx, args...)
}

func (c *client) ZRemRangeByLex(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{CommandZRemRangeByLex, key, min, max}
	return c.Int64(ctx, args...)
}

func (c *client) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) (int64, error) {
	args := []interface{}{CommandZRemRangeByRank, key, start, stop}
	return c.Int64(ctx, args...)
}

func (c *client) ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{CommandZRemRangeByScore, key, min, max}
	return c.Int64(ctx, args...)
}

func (c *client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{CommandZRevRange, key, start, stop}
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{CommandZRevRange, key, start, stop, "WITHSCORES"}
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRevRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{CommandZRevRangeByLex, key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRevRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{CommandZRevRangeByScore, key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

func (c *client) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	args := []interface{}{CommandZRevRank, key, member}
	return c.Int64(ctx, args...)
}

func (c *client) ZScore(ctx context.Context, key, member string) (float64, error) {
	args := []interface{}{CommandZScore, key, member}
	return c.Float64(ctx, args...)
}

func (c *client) ZUnion(ctx context.Context, args ...interface{}) ([]string, error) {
	args = append([]interface{}{CommandZUnion}, args...)
	return c.StringSlice(ctx, args...)
}

func (c *client) ZUnionWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	args = append([]interface{}{CommandZUnion}, args...)
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

func (c *client) ZUnionStore(ctx context.Context, dest string, args ...interface{}) (int64, error) {
	args = append([]interface{}{CommandZUnionStore, dest}, args...)
	return c.Int64(ctx, args...)
}
