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

package testcases

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

// ZADD
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 1 "uno"
// (integer) 1
// redis> ZADD myzset 2 "two" 3 "three"
// (integer) 2
// redis> ZRANGE myzset 0 -1 WITHSCORES
// 1) "one"
// 2) "1"
// 3) "uno"
// 4) "1"
// 5) "two"
// 6) "2"
// 7) "three"
// 8) "3"
// redis>

func ZAdd(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 1, "uno")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 2, "two", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(2))

	r4, err := c.ZRangeWithScores(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 4)
}

// ZCARD
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZCARD myzset
// (integer) 2
// redis>

func ZCard(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZCard(ctx, "myzset")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(2))
}

// ZCount
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZCOUNT myzset -inf +inf
// (integer) 3
// redis> ZCOUNT myzset (1 3
// (integer) 2
// redis>

func ZCount(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZCount(ctx, "myzset", "-inf", "+inf")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(3))

	r5, err := c.ZCount(ctx, "myzset", "(1", "3")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(2))
}

// ZDIFF
// redis> ZADD zset1 1 "one"
// (integer) 1
// redis> ZADD zset1 2 "two"
// (integer) 1
// redis> ZADD zset1 3 "three"
// (integer) 1
// redis> ZADD zset2 1 "one"
// (integer) 1
// redis> ZADD zset2 2 "two"
// (integer) 1
// redis> ZDIFF 2 zset1 zset2
// 1) "three"
// redis> ZDIFF 2 zset1 zset2 WITHSCORES
// 1) "three"
// 2) "3"
// redis>

func ZDiff(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "zset1", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "zset1", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "zset1", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZAdd(ctx, "zset2", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.ZAdd(ctx, "zset2", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.ZDiff(ctx, "zset1", "zset2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r6), 1)

	r7, err := c.ZDiffWithScores(ctx, "zset1", "zset2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r7), 1)
}

// ZINCRBY
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZINCRBY myzset 2 "one"
// "3"
// redis> ZRANGE myzset 0 -1 WITHSCORES
// 1) "two"
// 2) "2"
// 3) "one"
// 4) "3"
// redis>

func ZIncrBy(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZIncrBy(ctx, "myzset", 2, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, float64(3))

	r4, err := c.ZRangeWithScores(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 2)
}

// ZINTER
// redis> ZADD zset1 1 "one"
// (integer) 1
// redis> ZADD zset1 2 "two"
// (integer) 1
// redis> ZADD zset2 1 "one"
// (integer) 1
// redis> ZADD zset2 2 "two"
// (integer) 1
// redis> ZADD zset2 3 "three"
// (integer) 1
// redis> ZINTER 2 zset1 zset2
// 1) "one"
// 2) "two"
// redis> ZINTER 2 zset1 zset2 WITHSCORES
// 1) "one"
// 2) "2"
// 3) "two"
// 4) "4"
// redis>

func ZInter(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "zset1", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "zset1", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "zset2", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZAdd(ctx, "zset2", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.ZAdd(ctx, "zset2", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.ZInter(ctx, 2, "zset1", "zset2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r6), 2)

	r7, err := c.ZInterWithScores(ctx, 2, "zset1", "zset2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r7), 2)
}

// ZLEXCOUNT
// redis> ZADD myzset 0 a 0 b 0 c 0 d 0 e
// (integer) 5
// redis> ZADD myzset 0 f 0 g
// (integer) 2
// redis> ZLEXCOUNT myzset - +
// (integer) 7
// redis> ZLEXCOUNT myzset [b [f
// (integer) 5
// redis>

func ZLexCount(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(5))

	r2, err := c.ZAdd(ctx, "myzset", 0, "f", 0, "g")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(2))

	r3, err := c.ZLexCount(ctx, "myzset", "-", "+")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(r3))

	r4, err := c.ZLexCount(ctx, "myzset", "[b", "[f")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(5))
}

// ZMSCORE
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZMSCORE myzset "one" "two" "nofield"
// 1) "1"
// 2) "2"
// 3) (nil)
// redis>

func ZMScore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	// redis: unexpected type=<nil> for Float64
	r3, err := c.ZMScore(ctx, "myzset", "one", "two", "nofield")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r3), 2)
}

// ZPOPMAX
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZPOPMAX myzset
// 1) "three"
// 2) "3"
// redis>

func ZPopMax(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZPopMax(ctx, "myzset")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 1)
}

// ZPopMin
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZPOPMIN myzset
// 1) "one"
// 2) "1"
// redis>

func ZPopMin(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZPopMin(ctx, "myzset")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 1)

}

// ZRANDMEMBER
// redis> ZADD dadi 1 uno 2 due 3 tre 4 quattro 5 cinque 6 sei
// (integer) 6
// redis> ZRANDMEMBER dadi
// "uno"
// redis> ZRANDMEMBER dadi
// "tre"
// redis> ZRANDMEMBER dadi -5 WITHSCORES
// 1) "cinque"
//  2) "5"
//  3) "quattro"
//  4) "4"
//  5) "due"
//  6) "2"
//  7) "uno"
//  8) "1"
//  9) "quattro"
// 10) "4"
// redis>

func ZRandMember(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "dadi", 1, "uno", 2, "due", 3, "tre", 4, "quattro", 5, "cinque", 6, "sei")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(6))

	r2, err := c.ZRandMember(ctx, "dadi")
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, r2, "")

	r3, err := c.ZRandMember(ctx, "dadi")
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEqual(t, r3, "")

	r4, err := c.ZRandMemberWithScores(ctx, "dadi", -5)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 5)
}

// ZRange
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZRANGE myzset 0 -1
// 1) "one"
// 2) "two"
// 3) "three"
// redis> ZRANGE myzset 2 3
// 1) "three"
// redis> ZRANGE myzset -2 -1
// 1) "two"
// 2) "three"
// redis>

func ZRange(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRange(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 3)

	r5, err := c.ZRange(ctx, "myzset", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 1)

	r6, err := c.ZRange(ctx, "myzset", -2, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r6), 2)

	r7, err := c.ZRangeWithScores(ctx, "myzset", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r7), 2)
}

// ZRANGEBYLEX
// redis> ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g
// (integer) 7
// redis> ZRANGEBYLEX myzset - [c
// 1) "a"
// 2) "b"
// 3) "c"
// redis> ZRANGEBYLEX myzset - (c
// 1) "a"
// 2) "b"
// redis> ZRANGEBYLEX myzset [aaa (g
// 1) "b"
// 2) "c"
// 3) "d"
// 4) "e"
// 5) "f"
// redis>

func ZRangeByLex(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e", 0, "f", 0, "g")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(7))

	r2, err := c.ZRangeByLex(ctx, "myzset", "-", "[c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r2), 3)

	r3, err := c.ZRangeByLex(ctx, "myzset", "-", "(c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r3), 2)

	r4, err := c.ZRangeByLex(ctx, "myzset", "[aaa", "(g")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 5)
}

// ZRANGEBYSCORE
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZRANGEBYSCORE myzset -inf +inf
// 1) "one"
// 2) "two"
// 3) "three"
// redis> ZRANGEBYSCORE myzset 1 2
// 1) "one"
// 2) "two"
// redis> ZRANGEBYSCORE myzset (1 2
// 1) "two"
// redis> ZRANGEBYSCORE myzset (1 (2
// (empty list or set)
// redis>

func ZRangeByScore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRangeByScore(ctx, "myzset", "-inf", "+inf")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 3)

	r5, err := c.ZRangeByScore(ctx, "myzset", "1", "2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 2)

	r6, err := c.ZRangeByScore(ctx, "myzset", "(1", "2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r6), 1)

	r7, err := c.ZRangeByScore(ctx, "myzset", "(1", "(2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r7), 0)
}

// ZRank
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZRANK myzset "three"
// (integer) 2
// redis> ZRANK myzset "four"
// (nil)
// redis>

func ZRank(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRank(ctx, "myzset", "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(2))

	r5, err := c.ZRank(ctx, "myzset", "four")
	if err == redis.ErrNil {
		assert.Equal(t, r5, int64(-1))
	} else {
		t.Fatal(err)
	}

}

// ZRem
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZREM myzset "two"
// (integer) 1
// redis> ZRANGE myzset 0 -1 WITHSCORES
// 1) "one"
// 2) "1"
// 3) "three"
// 4) "3"
// redis>

func ZRem(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRem(ctx, "myzset", "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.ZRangeWithScores(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 2)
}

// ZREMRANGEBYLEX
// redis> ZADD myzset 0 aaaa 0 b 0 c 0 d 0 e
// (integer) 5
// redis> ZADD myzset 0 foo 0 zap 0 zip 0 ALPHA 0 alpha
// (integer) 5
// redis> ZRANGE myzset 0 -1
// 1) "ALPHA"
//  2) "aaaa"
//  3) "alpha"
//  4) "b"
//  5) "c"
//  6) "d"
//  7) "e"
//  8) "foo"
//  9) "zap"
// 10) "zip"
// redis> ZREMRANGEBYLEX myzset [alpha [omega
// (integer) 6
// redis> ZRANGE myzset 0 -1
// 1) "ALPHA"
// 2) "aaaa"

func ZRemRangeByLex(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 0, "aaaa", 0, "b", 0, "c", 0, "d", 0, "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(5))

	r2, err := c.ZAdd(ctx, "myzset", 0, "foo", 0, "zap", 0, "zip", 0, "ALPHA", 0, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(5))

	r3, err := c.ZRange(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r3), 10)

	r4, err := c.ZRemRangeByLex(ctx, "myzset", "[alpha", "[omega")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(6))

	r5, err := c.ZRange(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 4)
}

// ZRemRangeByRank
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZREMRANGEBYRANK myzset 0 1
// (integer) 2
// redis> ZRANGE myzset 0 -1 WITHSCORES
// 1) "three"
// 2) "3"
// redis>

func ZRemRangeByRank(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRemRangeByRank(ctx, "myzset", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(2))

	r5, err := c.ZRangeWithScores(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 1)
}

// ZRemRangeByScore
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZREMRANGEBYSCORE myzset -inf (2
// (integer) 1
// redis> ZRANGE myzset 0 -1 WITHSCORES
// 1) "two"
// 2) "2"
// 3) "three"
// 4) "3"
// redis>

func ZRemRangeByScore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRemRangeByScore(ctx, "myzset", "-inf", "(2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.ZRange(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 2)
}

// ZRevRange
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZREVRANGE myzset 0 -1
// 1) "three"
// 2) "two"
// 3) "one"
// redis> ZREVRANGE myzset 2 3
// 1) "one"
// redis> ZREVRANGE myzset -2 -1
// 1) "two"
// 2) "one"
// redis>

func ZRevRange(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRevRange(ctx, "myzset", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 3)

	r5, err := c.ZRevRange(ctx, "myzset", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 1)

	r6, err := c.ZRevRange(ctx, "myzset", -2, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r6), 2)
}

// ZRevRangeByLex
// redis> ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g
// (integer) 7
// redis> ZREVRANGEBYLEX myzset [c -
// 1) "c"
// 2) "b"
// 3) "a"
// redis> ZREVRANGEBYLEX myzset (c -
// 1) "b"
// 2) "a"
// redis> ZREVRANGEBYLEX myzset (g [aaa
// 1) "f"
// 2) "e"
// 3) "d"
// 4) "c"
// 5) "b"
// redis>

func ZRevRangeByLex(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e", 0, "f", 0, "g")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(7))

	r2, err := c.ZRevRangeByLex(ctx, "myzset", "[c", "-")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r2), 3)

	r3, err := c.ZRevRangeByLex(ctx, "myzset", "(c", "-")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r3), 2)

	r4, err := c.ZRevRangeByLex(ctx, "myzset", "(g", "[aaa")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 5)
}

// ZRevRangeByScore
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZREVRANGEBYSCORE myzset +inf -inf
// 1) "three"
// 2) "two"
// 3) "one"
// redis> ZREVRANGEBYSCORE myzset 2 1
// 1) "two"
// 2) "one"
// redis> ZREVRANGEBYSCORE myzset 2 (1
// 1) "two"
// redis> ZREVRANGEBYSCORE myzset (2 (1
// (empty list or set)
// redis>

func ZRevRangeByScore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRevRangeByScore(ctx, "myzset", "+inf", "-inf")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 3)

	r5, err := c.ZRevRangeByScore(ctx, "myzset", "2", "1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r5), 2)

	r6, err := c.ZRevRangeByScore(ctx, "myzset", "2", "(1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r6), 1)

	r7, err := c.ZRevRangeByScore(ctx, "myzset", "(2", "(1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r7), 0)
}

// ZRevRank
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 2 "two"
// (integer) 1
// redis> ZADD myzset 3 "three"
// (integer) 1
// redis> ZREVRANK myzset "one"
// (integer) 2
// redis> ZREVRANK myzset "four"
// (nil)
// redis>

func ZRevRank(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "myzset", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "myzset", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZRevRank(ctx, "myzset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(2))

	r5, err := c.ZRevRank(ctx, "myzset", "four")
	if err == redis.ErrNil {
		assert.Equal(t, r5, int64(-1))
	} else {
		t.Fatal(err)
	}

}

// ZScore
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZSCORE myzset "one"
// "1"
// redis>
func ZScore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "myzset", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZScore(ctx, "myzset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, float64(1))
}

// ZUNION
// redis> ZADD zset1 1 "one"
// (integer) 1
// redis> ZADD zset1 2 "two"
// (integer) 1
// redis> ZADD zset2 1 "one"
// (integer) 1
// redis> ZADD zset2 2 "two"
// (integer) 1
// redis> ZADD zset2 3 "three"
// (integer) 1
// redis> ZUNION 2 zset1 zset2
// 1) "one"
// 2) "three"
// 3) "two"
// redis>

func ZUnion(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "zset1", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "zset1", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "zset2", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZAdd(ctx, "zset2", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.ZAdd(ctx, "zset2", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.ZUnion(ctx, 2, "zset1", "zset2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r6), 3)
}

// ZUnionStore
// redis> ZADD zset1 1 "one"
// (integer) 1
// redis> ZADD zset1 2 "two"
// (integer) 1
// redis> ZADD zset2 1 "one"
// (integer) 1
// redis> ZADD zset2 2 "two"
// (integer) 1
// redis> ZADD zset2 3 "three"
// (integer) 1
// redis> ZUNIONSTORE out 2 zset1 zset2 WEIGHTS 2 3
// (integer) 3
// redis> ZRANGE out 0 -1 WITHSCORES
// 1) "one"
// 2) "5"
// 3) "three"
// 4) "9"
// 5) "two"
// 6) "10"
// redis>
func ZUnionStore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.ZAdd(ctx, "zset1", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.ZAdd(ctx, "zset1", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ZAdd(ctx, "zset2", 1, "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.ZAdd(ctx, "zset2", 2, "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.ZAdd(ctx, "zset2", 3, "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.ZUnionStore(ctx, "out", 2, "zset1", "zset2", "WEIGHTS", 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, int64(3))
}
