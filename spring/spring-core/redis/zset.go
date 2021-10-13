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
	"github.com/go-spring/spring-base/cast"
)

const (
	CommandZAdd             = "ZAdd"
	CommandZCard            = "ZCard"
	CommandZCount           = "ZCount"
	CommandZDiff            = "ZDiff"
	CommandZIncrBy          = "ZIncrBy"
	CommandZInter           = "ZInter"
	CommandZLexCount        = "ZLexCount"
	CommandZMScore          = "ZMScore"
	CommandZPopMax          = "ZPopMax"
	CommandZPopMin          = "ZPopMin"
	CommandZRandMember      = "ZRandMember"
	CommandZRange           = "ZRange"
	CommandZRangeByLex      = "ZRangeByLex"
	CommandZRemRangeByRank  = "ZRemRangeByRank"
	CommandZRemRangeByScore = "ZRemRangeByScore"
	CommandZRevRange        = "ZRevRange"
	CommandZRevRangeByLex   = "ZRevRangeByLex"
	CommandZRevRangeByScore = "ZRevRangeByScore"
	CommandZRevRank         = "ZRevRank"
	CommandZScore           = "ZScore"
	CommandZUnion           = "ZUnion"
	CommandZUnionStore      = "ZUnionStore"
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
	// Integer reply, the number of elements added to the
	// sorted set (excluding score updates).
	ZAdd(ctx context.Context, key string, members ...*ZItem) (int64, error)

	// ZCard https://redis.io/commands/zcard
	// Integer reply: the cardinality (number of elements)
	// of the sorted set, or 0 if key does not exist.
	ZCard(ctx context.Context, key string) (int64, error)

	// ZCount https://redis.io/commands/zcount
	// Integer reply: the number of elements in the specified score range.
	ZCount(ctx context.Context, key, min, max string) (int64, error)

	// ZDiff https://redis.io/commands/zdiff
	// Array reply: the result of the difference.
	ZDiff(ctx context.Context, keys ...string) ([]string, error)

	// ZDiffWithScores https://redis.io/commands/zdiff
	// Array reply: the result of the difference.
	ZDiffWithScores(ctx context.Context, keys ...string) ([]ZItem, error)

	// ZIncrBy https://redis.io/commands/zincrby
	// Bulk string reply: the new score of member (a double precision
	// floating point number), represented as string.
	ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error)

	// ZInter https://redis.io/commands/zinter
	// Array reply: the result of intersection.
	ZInter(ctx context.Context, store *ZStore) ([]string, error)

	// ZInterWithScores https://redis.io/commands/zinter
	// Array reply: the result of intersection.
	ZInterWithScores(ctx context.Context, store *ZStore) ([]ZItem, error)

	// ZLexCount https://redis.io/commands/zlexcount
	// Integer reply: the number of elements in the specified score range.
	ZLexCount(ctx context.Context, key, min, max string) (int64, error)

	// ZMScore https://redis.io/commands/zmscore
	// Array reply: list of scores or nil associated with the specified member
	// values (a double precision floating point number), represented as strings.
	ZMScore(ctx context.Context, key string, members ...string) ([]float64, error)

	// ZPopMax https://redis.io/commands/zpopmax
	// Array reply: list of popped elements and scores.
	ZPopMax(ctx context.Context, key string, count ...int64) ([]ZItem, error)

	// ZPopMin https://redis.io/commands/zpopmin
	// Array reply: list of popped elements and scores.
	ZPopMin(ctx context.Context, key string, count ...int64) ([]ZItem, error)

	// ZRandMember https://redis.io/commands/zrandmember
	// Bulk Reply with the randomly selected element, or nil when key does not exist.
	ZRandMember(ctx context.Context, key string, count int, withScores bool) ([]string, error)

	// ZRange https://redis.io/commands/zrange
	// Array reply: list of elements in the specified range.
	ZRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// ZRangeWithScores https://redis.io/commands/zrange
	// Array reply: list of elements in the specified range.
	ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZItem, error)

	// ZRangeByLex https://redis.io/commands/zrangebylex
	// Array reply: list of elements in the specified score range.
	ZRangeByLex(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

	// ZRangeByScore https://redis.io/commands/zrangebyscore
	// Array reply: list of elements in the specified score range.
	ZRangeByScore(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

	// ZRank https://redis.io/commands/zrank
	// If member exists in the sorted set, Integer reply: the rank of member.
	// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
	ZRank(ctx context.Context, key, member string) (int64, error)

	// ZRem https://redis.io/commands/zrem
	// Integer reply, The number of members removed from the sorted set, not including non existing members.
	ZRem(ctx context.Context, key string, members ...interface{}) (int64, error)

	// ZRemRangeByLex https://redis.io/commands/zremrangebylex
	// Integer reply: the number of elements removed.
	ZRemRangeByLex(ctx context.Context, key, min, max string) (int64, error)

	// ZRemRangeByRank https://redis.io/commands/zremrangebyrank
	// Integer reply: the number of elements removed.
	ZRemRangeByRank(ctx context.Context, key string, start, stop int64) (int64, error)

	// ZRemRangeByScore https://redis.io/commands/zremrangebyscore
	// Integer reply: the number of elements removed.
	ZRemRangeByScore(ctx context.Context, key, min, max string) (int64, error)

	// ZRevRange https://redis.io/commands/zrevrange
	// Array reply: list of elements in the specified range.
	ZRevRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// ZRevRangeByLex https://redis.io/commands/zrevrangebylex
	// Array reply: list of elements in the specified score range.
	ZRevRangeByLex(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

	// ZRevRangeByScore https://redis.io/commands/zrevrangebyscore
	// Array reply: list of elements in the specified score range.
	ZRevRangeByScore(ctx context.Context, key string, Min, Max string, Offset, Count int64) ([]string, error)

	// ZRevRank https://redis.io/commands/zrevrank
	// If member exists in the sorted set, Integer reply: the rank of member.
	// If member does not exist in the sorted set or key does not exist, Bulk string reply: nil.
	ZRevRank(ctx context.Context, key, member string) (int64, error)

	// ZScore https://redis.io/commands/zscore
	// Bulk string reply: the score of member (a double precision floating point number), represented as string.
	ZScore(ctx context.Context, key, member string) (float64, error)

	// ZUnion https://redis.io/commands/zunion
	// Array reply: the result of union.
	ZUnion(ctx context.Context, store ZStore) ([]string, error)

	// ZUnionStore https://redis.io/commands/zunionstore
	// Integer reply: the number of elements in the resulting sorted set at destination.
	ZUnionStore(ctx context.Context, dest string, store *ZStore) (int64, error)
}

const (
	ZAddOptionNone = ""
	ZAddOptionNX   = "NX"
	ZAddOptionXx   = "XX"
	ZAddOptionLt   = "LT"
	ZAddOptionGt   = "GT"
	ZAddOptionCh   = "CH"
)

func (c *BaseClient) ZAdd(ctx context.Context, key string, members ...*ZItem) (int64, error) {
	return c.zadd(ctx, ZAddOptionNone, key, members...)
}

func (c *BaseClient) ZAddNx(ctx context.Context, key string, members ...*ZItem) (int64, error) {
	return c.zadd(ctx, ZAddOptionNX, key, members...)
}

func (c *BaseClient) zadd(ctx context.Context, option, key string, members ...*ZItem) (int64, error) {

	args := []interface{}{CommandZAdd, key}

	if len(option) > 0 {
		args = append(args, option)
	}

	for _, item := range members {
		args = append(args, item.Score)
		args = append(args, item.Member)
	}

	return c.Int64(ctx, args...)
}

func (c *BaseClient) ZCard(ctx context.Context, key string) (int64, error) {
	args := []interface{}{CommandZCard, key}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) ZCount(ctx context.Context, key, min, max string) (int64, error) {
	args := []interface{}{CommandZCount, key}
	args = append(args, min)
	args = append(args, max)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) ZDiff(ctx context.Context, keys ...string) ([]string, error) {
	args := []interface{}{CommandZDiff}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.StringSlice(ctx, args...)
}

// ZDiffWithScores redis-server version >= 6.2.0.
func (c *BaseClient) ZDiffWithScores(ctx context.Context, keys ...string) ([]ZItem, error) {
	return nil, nil
}

func (c *BaseClient) ZIncrBy(ctx context.Context, key string, increment float64, member string) (float64, error) {
	args := []interface{}{CommandZIncrBy, key}
	args = append(args, increment)
	args = append(args, member)
	return c.Float64(ctx, args...)
}

// ZInter redis-server version >= 6.2.0.
func (c *BaseClient) ZInter(ctx context.Context, store *ZStore) ([]string, error) {
	args := []interface{}{CommandZInter}
	args = append(args, store.Aggregate)
	for _, key := range store.Keys {
		args = append(args, key)
	}

	// Weights
	if len(store.Weights) > 0 {
		args = append(args, "weights")
		for _, weight := range store.Weights {
			args = append(args, weight)
		}
	}

	return c.StringSlice(ctx, args...)
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
	args := []interface{}{CommandZRange, key}
	args = append(args, start)
	args = append(args, stop)
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) ZRangeWithScores(ctx context.Context, key string, start, stop int64) ([]ZItem, error) {
	args := []interface{}{CommandZRange, key}
	args = append(args, start)
	args = append(args, stop)
	result, err := c.StringSlice(ctx, args...)
	if err != nil {
		return nil, err
	}
	val := make([]ZItem, len(result)/2)
	for i := 0; i < len(val); i++ {
		member := result[i*2]
		score := cast.ToFloat64(result[i*2+1])
		val[i] = ZItem{
			Score:  score,
			Member: member,
		}
	}
	return val, nil
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
