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
	Score  float64
	Member interface{}
}

type ZStore struct {
	Keys      []string
	Weights   []float64
	Aggregate string
}

type ZSetCommand interface {

	// ZAdd https://redis.io/commands/zadd
	// Command: ZADD key [NX|XX] [GT|LT] [CH] [INCR] score member [score member ...]
	// Integer reply, the number of elements added to the
	// sorted set (excluding score updates).
	ZAdd(ctx context.Context, key string, members ...*ZItem) (int64, error)

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
	// Bulk string reply: the new score of member (a double precision
	// floating point number), represented as string.
	ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error)

	// ZInter https://redis.io/commands/zinter
	// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
	// Array reply: the result of intersection.
	ZInter(ctx context.Context, store *ZStore) ([]string, error)

	// ZInterWithScores https://redis.io/commands/zinter
	// Command: ZINTER numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX] [WITHSCORES]
	// Array reply: the result of intersection.
	ZInterWithScores(ctx context.Context, store *ZStore) ([]ZItem, error)

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
	ZPopMax(ctx context.Context, key string, count ...int64) ([]ZItem, error)

	// ZPopMin https://redis.io/commands/zpopmin
	// Command: ZPOPMIN key [count]
	// Array reply: list of popped elements and scores.
	ZPopMin(ctx context.Context, key string, count ...int64) ([]ZItem, error)

	// ZRandMember https://redis.io/commands/zrandmember
	// Command: ZRANDMEMBER key [count [WITHSCORES]]
	// Bulk Reply with the randomly selected element, or nil when key does not exist.
	ZRandMember(ctx context.Context, key string, count int, withScores bool) ([]string, error)

	// ZRange https://redis.io/commands/zrange
	// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
	// Array reply: list of elements in the specified range.
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// ZRangeWithScores https://redis.io/commands/zrange
	// Command: ZRANGE key min max [BYSCORE|BYLEX] [REV] [LIMIT offset count] [WITHSCORES]
	// Array reply: list of elements in the specified range.
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZItem, error)

	// ZRangeByLex https://redis.io/commands/zrangebylex
	// Command: ZRANGEBYLEX key min max [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRangeByLex(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

	// ZRangeByScore https://redis.io/commands/zrangebyscore
	// Command: ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRangeByScore(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

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

	// ZRevRangeByLex https://redis.io/commands/zrevrangebylex
	// Command: ZREVRANGEBYLEX key max min [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRevRangeByLex(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

	// ZRevRangeByScore https://redis.io/commands/zrevrangebyscore
	// Command: ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT offset count]
	// Array reply: list of elements in the specified score range.
	ZRevRangeByScore(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

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
	ZUnion(ctx context.Context, store ZStore) ([]string, error)

	// ZUnionStore https://redis.io/commands/zunionstore
	// Command: ZUNIONSTORE destination numkeys key [key ...] [WEIGHTS weight [weight ...]] [AGGREGATE SUM|MIN|MAX]
	// Integer reply: the number of elements in the resulting sorted set at destination.
	ZUnionStore(ctx context.Context, dest string, store *ZStore) (int64, error)
}

func (c *BaseClient) ZAdd(ctx context.Context, key string, members ...*ZItem) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZCard(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZDiff(ctx context.Context, keys ...string) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZDiffWithScores(ctx context.Context, keys ...string) ([]ZItem, error) {
	return nil, nil
}

func (c *BaseClient) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	return 0, nil
}

func (c *BaseClient) ZInter(ctx context.Context, store *ZStore) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZInterWithScores(ctx context.Context, store *ZStore) ([]ZItem, error) {
	return nil, nil
}

func (c *BaseClient) ZLexCount(ctx context.Context, key, min, max string) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZMScore(ctx context.Context, key string, members ...string) ([]float64, error) {
	return nil, nil
}

func (c *BaseClient) ZPopMax(ctx context.Context, key string, count ...int64) ([]ZItem, error) {
	return nil, nil
}

func (c *BaseClient) ZPopMin(ctx context.Context, key string, count ...int64) ([]ZItem, error) {
	return nil, nil
}

func (c *BaseClient) ZRandMember(ctx context.Context, key string, count int, withScores bool) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZItem, error) {
	return nil, nil
}

func (c *BaseClient) ZRangeByLex(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZRangeByScore(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZRank(ctx context.Context, key, member string) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZRemRangeByLex(ctx context.Context, key, min, max string) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZRemRangeByRank(ctx context.Context, key string, start, stop int64) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZRevRangeByLex(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZRevRangeByScore(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZRevRank(ctx context.Context, key, member string) (int64, error) {
	return 0, nil
}

func (c *BaseClient) ZScore(ctx context.Context, key, member string) (float64, error) {
	return 0, nil
}

func (c *BaseClient) ZUnion(ctx context.Context, store ZStore) ([]string, error) {
	return nil, nil
}

func (c *BaseClient) ZUnionStore(ctx context.Context, dest string, store *ZStore) (int64, error) {
	return 0, nil
}
