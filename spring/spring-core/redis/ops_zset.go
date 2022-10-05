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

// ZAdd https://redis.io/commands/zadd
// Command: ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
// Integer reply, the number of elements added to the
// sorted set (excluding score updates).
func (c *Client) ZAdd(ctx context.Context, key string, args ...interface{}) (int64, error) {
	args = append([]interface{}{"ZADD", key}, args...)
	return c.Int(ctx, args...)
}

// ZCard https://redis.io/commands/zcard
// Command: ZCARD key
// Integer reply: the cardinality (number of elements)
// of the sorted set, or 0 if key does not exist.
func (c *Client) ZCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{"ZCARD", key}
	return c.Int(ctx, args...)
}

// ZCount https://redis.io/commands/zcount
// Command: ZCOUNT key min max
// Integer reply: the number of elements in the specified score range.
func (c *Client) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{"ZCOUNT", key, min, max}
	return c.Int(ctx, args...)
}

// ZDiff https://redis.io/commands/zdiff
// Command: ZDIFF numkeys key [key ...] [WITHSCORES]
// Array reply: the result of the difference.
func (c *Client) ZDiff(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{"ZDIFF", len(keys)}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

// ZDiffWithScores https://redis.io/commands/zdiff
// Command: ZDIFF numkeys key [key ...] [WITHSCORES]
// Array reply: the result of the difference.
func (c *Client) ZDiffWithScores(ctx context.Context, keys ...string) ([]ZItem, error) {
	args := []interface{}{"ZDIFF", len(keys)}
	for _, key := range keys {
		args = append(args, key)
	}
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

// ZIncrBy https://redis.io/commands/zincrby
// Command: ZINCRBY key increment member
// Bulk string reply: the new score of member
// (a double precision floating point number), represented as string.
func (c *Client) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	args := []interface{}{"ZINCRBY", key, increment, member}
	return c.Float(ctx, args...)
}

// ZInter https://redis.io/commands/zinter
// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of intersection.
func (c *Client) ZInter(ctx context.Context, args ...interface{}) ([]string, error) {
	args = append([]interface{}{"ZINTER"}, args...)
	return c.StringSlice(ctx, args...)
}

// ZInterWithScores https://redis.io/commands/zinter
// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of intersection.
func (c *Client) ZInterWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	args = append([]interface{}{"ZINTER"}, args...)
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

// ZLexCount https://redis.io/commands/zlexcount
// Command: ZLEXCOUNT key min max
// Integer reply: the number of elements in the specified score range.
func (c *Client) ZLexCount(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{"ZLEXCOUNT", key, min, max}
	return c.Int(ctx, args...)
}

// ZMScore https://redis.io/commands/zmscore
// Command: ZMSCORE key member [member ...]
// Array reply: list of scores or nil associated with the specified member
// values (a double precision floating point number), represented as strings.
func (c *Client) ZMScore(ctx context.Context, key string, members ...string) ([]float64, error) {
	args := []interface{}{"ZMSCORE", key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.FloatSlice(ctx, args...)
}

// ZPopMax https://redis.io/commands/zpopmax
// Command: ZPOPMAX key [count]
// Array reply: list of popped elements and scores.
func (c *Client) ZPopMax(ctx context.Context, key string) ([]ZItem, error) {
	args := []interface{}{"ZPOPMAX", key}
	return c.ZItemSlice(ctx, args...)
}

// ZPopMaxN https://redis.io/commands/zpopmax
// Command: ZPOPMAX key [count]
// Array reply: list of popped elements and scores.
func (c *Client) ZPopMaxN(ctx context.Context, key string, count int64) ([]ZItem, error) {
	args := []interface{}{"ZPOPMAX", key, count}
	return c.ZItemSlice(ctx, args...)
}

// ZPopMin https://redis.io/commands/zpopmin
// Command: ZPOPMIN key [count]
// Array reply: list of popped elements and scores.
func (c *Client) ZPopMin(ctx context.Context, key string) ([]ZItem, error) {
	args := []interface{}{"ZPOPMIN", key}
	return c.ZItemSlice(ctx, args...)
}

// ZPopMinN https://redis.io/commands/zpopmin
// Command: ZPOPMIN key [count]
// Array reply: list of popped elements and scores.
func (c *Client) ZPopMinN(ctx context.Context, key string, count int64) ([]ZItem, error) {
	args := []interface{}{"ZPOPMIN", key, count}
	return c.ZItemSlice(ctx, args...)
}

// ZRandMember https://redis.io/commands/zrandmember
// Command: ZRANDMEMBER key [count [WITHSCORES]]
// Bulk Reply with the randomly selected element, or nil when key does not exist.
func (c *Client) ZRandMember(ctx context.Context, key string) (string, error) {
	args := []interface{}{"ZRANDMEMBER", key}
	return c.String(ctx, args...)
}

// ZRandMemberN https://redis.io/commands/zrandmember
// Command: ZRANDMEMBER key [count [WITHSCORES]]
// Bulk Reply with the randomly selected element, or nil when key does not exist.
func (c *Client) ZRandMemberN(ctx context.Context, key string, count int) ([]string, error) {
	args := []interface{}{"ZRANDMEMBER", key, count}
	return c.StringSlice(ctx, args...)
}

// ZRandMemberWithScores https://redis.io/commands/zrandmember
// Command: ZRANDMEMBER key [count [WITHSCORES]]
// Bulk Reply with the randomly selected element, or nil when key does not exist.
func (c *Client) ZRandMemberWithScores(ctx context.Context, key string, count int) ([]ZItem, error) {
	args := []interface{}{"ZRANDMEMBER", key, count, "WITHSCORES"}
	return c.ZItemSlice(ctx, args...)
}

// ZRange https://redis.io/commands/zrange
// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *Client) ZRange(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]string, error) {
	args = append([]interface{}{"ZRANGE", key, start, stop}, args...)
	return c.StringSlice(ctx, args...)
}

// ZRangeWithScores https://redis.io/commands/zrange
// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *Client) ZRangeWithScores(ctx context.Context, key string, start, stop int64, args ...interface{}) ([]ZItem, error) {
	args = append([]interface{}{"ZRANGE", key, start, stop}, args...)
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

// ZRangeByLex https://redis.io/commands/zrangebylex
// Command: ZRANGEBYLEX key min max [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *Client) ZRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{"ZRANGEBYLEX", key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

// ZRangeByScore https://redis.io/commands/zrangebyscore
// Command: ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *Client) ZRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{"ZRANGEBYSCORE", key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

// ZRank https://redis.io/commands/zrank
// Command: ZRANK key member
// If member exists in the sorted set, Integer reply: the rank of member.
// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
func (c *Client) ZRank(ctx context.Context, key, member string) (int64, error) {
	args := []interface{}{"ZRANK", key, member}
	return c.Int(ctx, args...)
}

// ZRem https://redis.io/commands/zrem
// Command: ZREM key member [member ...]
// Integer reply, The number of members removed from the sorted set, not including non existing members.
func (c *Client) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{"ZREM", key}
	for _, member := range members {
		args = append(args, member)
	}
	return c.Int(ctx, args...)
}

// ZRemRangeByLex https://redis.io/commands/zremrangebylex
// Command: ZREMRANGEBYLEX key min max
// Integer reply: the number of elements removed.
func (c *Client) ZRemRangeByLex(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{"ZREMRANGEBYLEX", key, min, max}
	return c.Int(ctx, args...)
}

// ZRemRangeByRank https://redis.io/commands/zremrangebyrank
// Command: ZREMRANGEBYRANK key start stop
// Integer reply: the number of elements removed.
func (c *Client) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) (int64, error) {
	args := []interface{}{"ZREMRANGEBYRANK", key, start, stop}
	return c.Int(ctx, args...)
}

// ZRemRangeByScore https://redis.io/commands/zremrangebyscore
// Command: ZREMRANGEBYSCORE key min max
// Integer reply: the number of elements removed.
func (c *Client) ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{"ZREMRANGEBYSCORE", key, min, max}
	return c.Int(ctx, args...)
}

// ZRevRange https://redis.io/commands/zrevrange
// Command: ZREVRANGE key start stop [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *Client) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{"ZREVRANGE", key, start, stop}
	return c.StringSlice(ctx, args...)
}

// ZRevRangeWithScores https://redis.io/commands/zrevrange
// Command: ZREVRANGE key start stop [WITHSCORES]
// Array reply: list of elements in the specified range.
func (c *Client) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) ([]string, error) {
	args := []interface{}{"ZREVRANGE", key, start, stop, "WITHSCORES"}
	return c.StringSlice(ctx, args...)
}

// ZRevRangeByLex https://redis.io/commands/zrevrangebylex
// Command: ZREVRANGEBYLEX key max min [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *Client) ZRevRangeByLex(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{"ZREVRANGEBYLEX", key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

// ZRevRangeByScore https://redis.io/commands/zrevrangebyscore
// Command: ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]
// Array reply: list of elements in the specified score range.
func (c *Client) ZRevRangeByScore(ctx context.Context, key string, min, max string, args ...interface{}) ([]string, error) {
	args = append([]interface{}{"ZREVRANGEBYSCORE", key, min, max}, args...)
	return c.StringSlice(ctx, args...)
}

// ZRevRank https://redis.io/commands/zrevrank
// Command: ZREVRANK key member
// If member exists in the sorted set, Integer reply: the rank of member.
// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
func (c *Client) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	args := []interface{}{"ZREVRANK", key, member}
	return c.Int(ctx, args...)
}

// ZScore https://redis.io/commands/zscore
// Command: ZSCORE key member
// Bulk string reply: the score of member (a double precision floating point number), represented as string.
func (c *Client) ZScore(ctx context.Context, key, member string) (float64, error) {
	args := []interface{}{"ZSCORE", key, member}
	return c.Float(ctx, args...)
}

// ZUnion https://redis.io/commands/zunion
// Command: ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of union.
func (c *Client) ZUnion(ctx context.Context, args ...interface{}) ([]string, error) {
	args = append([]interface{}{"ZUNION"}, args...)
	return c.StringSlice(ctx, args...)
}

// ZUnionWithScores https://redis.io/commands/zunion
// Command: ZUNION numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
// Array reply: the result of union.
func (c *Client) ZUnionWithScores(ctx context.Context, args ...interface{}) ([]ZItem, error) {
	args = append([]interface{}{"ZUNION"}, args...)
	args = append(args, "WITHSCORES")
	return c.ZItemSlice(ctx, args...)
}

// ZUnionStore https://redis.io/commands/zunionstore
// Command: ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
// Integer reply: the number of elements in the resulting sorted set at destination.
func (c *Client) ZUnionStore(ctx context.Context, dest string, args ...interface{}) (int64, error) {
	args = append([]interface{}{"ZUNIONSTORE", dest}, args...)
	return c.Int(ctx, args...)
}
