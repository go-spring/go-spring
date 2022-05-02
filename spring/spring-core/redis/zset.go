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

type ZItem struct {
	Member interface{}
	Score  float64
}

type ZSetOperations struct {
	c *Client
}

func NewZSetOperations(c *Client) *ZSetOperations {
	return &ZSetOperations{c: c}
}

// ZAdd https://redis.io/commands/zadd
// Command: ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
// Integer reply, the number of elements added to the
// sorted set (excluding score updates).
func (c *ZSetOperations) ZAdd(ctx context.Context, key string, args ...interface{}) (int64, error) {
	args = append([]interface{}{key}, args...)
	return c.c.Int(ctx, "ZADD", args...)
}

// ZCard https://redis.io/commands/zcard
// Command: ZCARD key
// Integer reply: the cardinality (number of elements)
// of the sorted set, or 0 if key does not exist.
func (c *ZSetOperations) ZCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{key}
	return c.c.Int(ctx, "ZCARD", args...)
}

// ZCount https://redis.io/commands/zcount
// Command: ZCOUNT key min max
// Integer reply: the number of elements in the specified score range.
func (c *ZSetOperations) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{key, min, max}
	return c.c.Int(ctx, "ZCOUNT", args...)
}

// ZDiff https://redis.io/commands/zdiff
// Command: ZDIFF numkeys key [key ...] [WITHSCORES]
// Array reply: the result of the difference.
func (c *ZSetOperations) ZDiff(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{len(keys)}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.c.StringSlice(ctx, "ZDIFF", args...)
}

// ZDiffWithScores https://redis.io/commands/zdiff
// Command: ZDIFF numkeys key [key ...] [WITHSCORES]
// Array reply: the result of the difference.
func (c *ZSetOperations) ZDiffWithScores(ctx context.Context, keys ...string) ([]ZItem, error) {
	args := []interface{}{len(keys)}
	for _, key := range keys {
		args = append(args, key)
	}
	args = append(args, "WITHSCORES")
	return c.c.ZItemSlice(ctx, "ZDIFF", args...)
}

// ZIncrBy https://redis.io/commands/zincrby
// Command: ZINCRBY key increment member
// Bulk string reply: the new score of member
// (a double precision floating point number), represented as string.
func (c *ZSetOperations) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	args := []interface{}{key, increment, member}
	return c.c.Float(ctx, "ZINCRBY", args...)
}

// ZInter https://redis.io/commands/zinter
// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of intersection.
func (c *ZSetOperations) ZInter(ctx context.Context, args ...interface{}) ([]string, error) {
	return c.c.StringSlice(ctx, "ZINTER", args...)
}

// ZInterWithScores https://redis.io/commands/zinter
// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of intersection.
func (c *ZSetOperations) ZInterWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	args = append(args, "WITHSCORES")
	return c.c.ZItemSlice(ctx, "ZINTER", args...)
}

// ZLexCount https://redis.io/commands/zlexcount
// Command: ZLEXCOUNT key min max
// Integer reply: the number of elements in the specified score range.
func (c *ZSetOperations) ZLexCount(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{key, min, max}
	return c.c.Int(ctx, "ZLEXCOUNT", args...)
}

// ZMScore https://redis.io/commands/zmscore
// Command: ZMSCORE key member [member ...]
// Array reply: list of scores or nil associated with the specified member
// values (a double precision floating point number), represented as strings.
func (c *ZSetOperations) ZMScore(ctx context.Context, key string, members ...string) ([]float64, error) {
	args := []interface{}{key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.c.FloatSlice(ctx, "ZMSCORE", args...)
}

// ZPopMax https://redis.io/commands/zpopmax
// Command: ZPOPMAX key [count]
// Array reply: list of popped elements and scores.
func (c *ZSetOperations) ZPopMax(ctx context.Context, key string) ([]ZItem, error) {
	args := []interface{}{key}
	return c.c.ZItemSlice(ctx, "ZPOPMAX", args...)
}

// ZPopMaxN https://redis.io/commands/zpopmax
// Command: ZPOPMAX key [count]
// Array reply: list of popped elements and scores.
func (c *ZSetOperations) ZPopMaxN(ctx context.Context, key string, count int64) ([]ZItem, error) {
	args := []interface{}{key, count}
	return c.c.ZItemSlice(ctx, "ZPOPMAX", args...)
}

// ZPopMin https://redis.io/commands/zpopmin
// Command: ZPOPMIN key [count]
// Array reply: list of popped elements and scores.
func (c *ZSetOperations) ZPopMin(ctx context.Context, key string) ([]ZItem, error) {
	args := []interface{}{key}
	return c.c.ZItemSlice(ctx, "ZPOPMIN", args...)
}

// ZPopMinN https://redis.io/commands/zpopmin
// Command: ZPOPMIN key [count]
// Array reply: list of popped elements and scores.
func (c *ZSetOperations) ZPopMinN(ctx context.Context, key string, count int64) ([]ZItem, error) {
	args := []interface{}{key, count}
	return c.c.ZItemSlice(ctx, "ZPOPMIN", args...)
}

// ZRandMember https://redis.io/commands/zrandmember
// Command: ZRANDMEMBER key [count [WITHSCORES]]
// Bulk Reply with the randomly selected element, or nil when key does not exist.
func (c *ZSetOperations) ZRandMember(ctx context.Context, key string) (string, error) {
	args := []interface{}{key}
	return c.c.String(ctx, "ZRANDMEMBER", args...)
}

// ZRandMemberN https://redis.io/commands/zrandmember
// Command: ZRANDMEMBER key [count [WITHSCORES]]
// Bulk Reply with the randomly selected element, or nil when key does not exist.
func (c *ZSetOperations) ZRandMemberN(ctx context.Context, key string, count int) ([]string, error) {
	args := []interface{}{key, count}
	return c.c.StringSlice(ctx, "ZRANDMEMBER", args...)
}

// ZRandMemberWithScores https://redis.io/commands/zrandmember
// Command: ZRANDMEMBER key [count [WITHSCORES]]
// Bulk Reply with the randomly selected element, or nil when key does not exist.
func (c *ZSetOperations) ZRandMemberWithScores(ctx context.Context, key string, count int) ([]ZItem, error) {
	args := []interface{}{key, count, "WITHSCORES"}
	return c.c.ZItemSlice(ctx, "ZRANDMEMBER", args...)
}

// ZRange https://redis.io/commands/zrange
// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *ZSetOperations) ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]string, error) {
	args = append([]interface{}{key, start, stop}, args...)
	return c.c.StringSlice(ctx, "ZRANGE", args...)
}

// ZRangeWithScores https://redis.io/commands/zrange
// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *ZSetOperations) ZRangeWithScores(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]ZItem, error) {
	args = append([]interface{}{key, start, stop}, args...)
	args = append(args, "WITHSCORES")
	return c.c.ZItemSlice(ctx, "ZRANGE", args...)
}

// ZRangeByLex https://redis.io/commands/zrangebylex
// Command: ZRANGEBYLEX key min max [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *ZSetOperations) ZRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{key, min, max}, args...)
	return c.c.StringSlice(ctx, "ZRANGEBYLEX", args...)
}

// ZRangeByScore https://redis.io/commands/zrangebyscore
// Command: ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *ZSetOperations) ZRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{key, min, max}, args...)
	return c.c.StringSlice(ctx, "ZRANGEBYSCORE", args...)
}

// ZRank https://redis.io/commands/zrank
// Command: ZRANK key member
// If member exists in the sorted set, Integer reply: the rank of member.
// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
func (c *ZSetOperations) ZRank(ctx context.Context, key, member string) (int64, error) {
	args := []interface{}{key, member}
	return c.c.Int(ctx, "ZRANK", args...)
}

// ZRem https://redis.io/commands/zrem
// Command: ZREM key member [member ...]
// Integer reply, The number of members removed from the sorted set, not including non existing members.
func (c *ZSetOperations) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.c.Int(ctx, "ZREM", args...)
}

// ZRemRangeByLex https://redis.io/commands/zremrangebylex
// Command: ZREMRANGEBYLEX key min max
// Integer reply: the number of elements removed.
func (c *ZSetOperations) ZRemRangeByLex(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{key, min, max}
	return c.c.Int(ctx, "ZREMRANGEBYLEX", args...)
}

// ZRemRangeByRank https://redis.io/commands/zremrangebyrank
// Command: ZREMRANGEBYRANK key start stop
// Integer reply: the number of elements removed.
func (c *ZSetOperations) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) (int64, error) {
	args := []interface{}{key, start, stop}
	return c.c.Int(ctx, "ZREMRANGEBYRANK", args...)
}

// ZRemRangeByScore https://redis.io/commands/zremrangebyscore
// Command: ZREMRANGEBYSCORE key min max
// Integer reply: the number of elements removed.
func (c *ZSetOperations) ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{key, min, max}
	return c.c.Int(ctx, "ZREMRANGEBYSCORE", args...)
}

// ZRevRange https://redis.io/commands/zrevrange
// Command: ZREVRANGE key start stop [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *ZSetOperations) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{key, start, stop}
	return c.c.StringSlice(ctx, "ZREVRANGE", args...)
}

// ZRevRangeWithScores https://redis.io/commands/zrevrange
// Command: ZREVRANGE key start stop [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *ZSetOperations) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{key, start, stop, "WITHSCORES"}
	return c.c.StringSlice(ctx, "ZREVRANGE", args...)
}

// ZRevRangeByLex https://redis.io/commands/zrevrangebylex
// Command: ZREVRANGEBYLEX key max min [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *ZSetOperations) ZRevRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{key, min, max}, args...)
	return c.c.StringSlice(ctx, "ZREVRANGEBYLEX", args...)
}

// ZRevRangeByScore https://redis.io/commands/zrevrangebyscore
// Command: ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *ZSetOperations) ZRevRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{key, min, max}, args...)
	return c.c.StringSlice(ctx, "ZREVRANGEBYSCORE", args...)
}

// ZRevRank https://redis.io/commands/zrevrank
// Command: ZREVRANK key member
// If member exists in the sorted set, Integer reply: the rank of member.
// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
func (c *ZSetOperations) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	args := []interface{}{key, member}
	return c.c.Int(ctx, "ZREVRANK", args...)
}

// ZScore https://redis.io/commands/zscore
// Command: ZSCORE key member
// Bulk string reply: the score of member (a double precision floating point number), represented as string.
func (c *ZSetOperations) ZScore(ctx context.Context, key, member string) (float64, error) {
	args := []interface{}{key, member}
	return c.c.Float(ctx, "ZSCORE", args...)
}

// ZUnion https://redis.io/commands/zunion
// Command: ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of union.
func (c *ZSetOperations) ZUnion(ctx context.Context, args ...interface{}) ([]string, error) {
	return c.c.StringSlice(ctx, "ZUNION", args...)
}

// ZUnionWithScores https://redis.io/commands/zunion
// Command: ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of union.
func (c *ZSetOperations) ZUnionWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	args = append(args, "WITHSCORES")
	return c.c.ZItemSlice(ctx, "ZUNION", args...)
}

// ZUnionStore https://redis.io/commands/zunionstore
// Command: ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
// Integer reply: the number of elements in the resulting sorted set at destination.
func (c *ZSetOperations) ZUnionStore(ctx context.Context, dest string, args ...interface{}) (int64, error) {
	args = append([]interface{}{dest}, args...)
	return c.c.Int(ctx, "ZUNIONSTORE", args...)
}
