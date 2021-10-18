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
