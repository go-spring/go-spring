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

package cases

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

var ZAdd = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []redis.ZItem{{"one", 1}, {"uno", 1}, {"two", 2}, {"three", 3}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 1 uno",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two 3 three",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1 WITHSCORES",
			"response": ["one", "1", "uno", "1", "two", "2", "three", "3"]
		}]
	}`,
}

var ZCard = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZCARD myzset",
			"response": 2
		}]
	}`,
}

var ZCount = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZCOUNT myzset -inf +inf",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "ZCOUNT myzset (1 3",
			"response": 2
		}]
	}`,
}

var ZDiff = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r6, []string{"three"})

		r7, err := c.ZDiffWithScores(ctx, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"three", 3}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD zset1 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset1 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset1 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZDIFF 2 zset1 zset2",
			"response": ["three"]
		}, {
			"protocol": "redis",
			"request": "ZDIFF 2 zset1 zset2 WITHSCORES",
			"response": ["three", "3"]
		}]
	}`,
}

var ZIncrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []redis.ZItem{{"two", 2}, {"one", 3}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZINCRBY myzset 2 one",
			"response": "3"
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1 WITHSCORES",
			"response": ["two", "2", "one", "3"]
		}]
	}`,
}

var ZInter = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r6, []string{"one", "two"})

		r7, err := c.ZInterWithScores(ctx, 2, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 2}, {"two", 4}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD zset1 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset1 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZINTER 2 zset1 zset2",
			"response": ["one", "two"]
		}, {
			"protocol": "redis",
			"request": "ZINTER 2 zset1 zset2 WITHSCORES",
			"response": ["one", "2", "two", "4"]
		}]
	}`,
}

var ZLexCount = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r3, int64(7))

		r4, err := c.ZLexCount(ctx, "myzset", "[b", "[f")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(5))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 0 f 0 g",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "ZLEXCOUNT myzset - +",
			"response": 7
		}, {
			"protocol": "redis",
			"request": "ZLEXCOUNT myzset [b [f",
			"response": 5
		}]
	}`,
}

var ZMScore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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

		r3, err := c.ZMScore(ctx, "myzset", "one", "two", "nofield")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []float64{1, 2, 0})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZMSCORE myzset one two nofield",
			"response": ["1", "2", null]
		}]
	}`,
}

var ZPopMax = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []redis.ZItem{{"three", 3}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZPOPMAX myzset",
			"response": ["three", "3"]
		}]
	}`,
}

var ZPopMin = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []redis.ZItem{{"one", 1}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZPOPMIN myzset",
			"response": ["one", "1"]
		}]
	}`,
}

var ZRandMember = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD dadi 1 uno 2 due 3 tre 4 quattro 5 cinque 6 sei",
			"response": 6
		}, {
			"protocol": "redis",
			"request": "ZRANDMEMBER dadi",
			"response": "sei"
		}, {
			"protocol": "redis",
			"request": "ZRANDMEMBER dadi",
			"response": "sei"
		}, {
			"protocol": "redis",
			"request": "ZRANDMEMBER dadi -5 WITHSCORES",
			"response": ["uno", "1", "uno", "1", "cinque", "5", "sei", "6", "due", "2"]
		}]
	}`,
}

var ZRange = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []string{"one", "two", "three"})

		r5, err := c.ZRange(ctx, "myzset", 2, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"three"})

		r6, err := c.ZRange(ctx, "myzset", -2, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two", "three"})

		r7, err := c.ZRangeWithScores(ctx, "myzset", 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 1}, {"two", 2}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1",
			"response": ["one", "two", "three"]
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 2 3",
			"response": ["three"]
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset -2 -1",
			"response": ["two", "three"]
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 1 WITHSCORES",
			"response": ["one", "1", "two", "2"]
		}]
	}`,
}

var ZRangeByLex = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e", 0, "f", 0, "g")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(7))

		r2, err := c.ZRangeByLex(ctx, "myzset", "-", "[c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, []string{"a", "b", "c"})

		r3, err := c.ZRangeByLex(ctx, "myzset", "-", "(c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"a", "b"})

		r4, err := c.ZRangeByLex(ctx, "myzset", "[aaa", "(g")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"b", "c", "d", "e", "f"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g",
			"response": 7
		}, {
			"protocol": "redis",
			"request": "ZRANGEBYLEX myzset - [c",
			"response": ["a", "b", "c"]
		}, {
			"protocol": "redis",
			"request": "ZRANGEBYLEX myzset - (c",
			"response": ["a", "b"]
		}, {
			"protocol": "redis",
			"request": "ZRANGEBYLEX myzset [aaa (g",
			"response": ["b", "c", "d", "e", "f"]
		}]
	}`,
}

var ZRangeByScore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []string{"one", "two", "three"})

		r5, err := c.ZRangeByScore(ctx, "myzset", "1", "2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one", "two"})

		r6, err := c.ZRangeByScore(ctx, "myzset", "(1", "2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two"})

		r7, err := c.ZRangeByScore(ctx, "myzset", "(1", "(2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(r7), 0)
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZRANGEBYSCORE myzset -inf +inf",
			"response": ["one", "two", "three"]
		}, {
			"protocol": "redis",
			"request": "ZRANGEBYSCORE myzset 1 2",
			"response": ["one", "two"]
		}, {
			"protocol": "redis",
			"request": "ZRANGEBYSCORE myzset (1 2",
			"response": ["two"]
		}, {
			"protocol": "redis",
			"request": "ZRANGEBYSCORE myzset (1 (2",
			"response": []
		}]
	}`,
}

var ZRank = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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

		_, err = c.ZRank(ctx, "myzset", "four")
		assert.True(t, redis.IsErrNil(err))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZRANK myzset three",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "ZRANK myzset four",
			"response": "(nil)"
		}]
	}`,
}

var ZRem = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r5, []redis.ZItem{{"one", 1}, {"three", 3}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZREM myzset two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1 WITHSCORES",
			"response": ["one", "1", "three", "3"]
		}]
	}`,
}

var ZRemRangeByLex = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r3, []string{
			"ALPHA", "aaaa", "alpha", "b", "c", "d", "e", "foo", "zap", "zip",
		})

		r4, err := c.ZRemRangeByLex(ctx, "myzset", "[alpha", "[omega")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(6))

		r5, err := c.ZRange(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"ALPHA", "aaaa", "zap", "zip"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 0 aaaa 0 b 0 c 0 d 0 e",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 0 foo 0 zap 0 zip 0 ALPHA 0 alpha",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1",
			"response": ["ALPHA", "aaaa", "alpha", "b", "c", "d", "e", "foo", "zap", "zip"]
		}, {
			"protocol": "redis",
			"request": "ZREMRANGEBYLEX myzset [alpha [omega",
			"response": 6
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1",
			"response": ["ALPHA", "aaaa", "zap", "zip"]
		}]
	}`,
}

var ZRemRangeByRank = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r5, []redis.ZItem{{"three", 3}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZREMRANGEBYRANK myzset 0 1",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1 WITHSCORES",
			"response": ["three", "3"]
		}]
	}`,
}

var ZRemRangeByScore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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

		r5, err := c.ZRangeWithScores(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []redis.ZItem{{"two", 2}, {"three", 3}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZREMRANGEBYSCORE myzset -inf (2",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZRANGE myzset 0 -1 WITHSCORES",
			"response": ["two", "2", "three", "3"]
		}]
	}`,
}

var ZRevRange = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []string{"three", "two", "one"})

		r5, err := c.ZRevRange(ctx, "myzset", 2, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one"})

		r6, err := c.ZRevRange(ctx, "myzset", -2, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two", "one"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZREVRANGE myzset 0 -1",
			"response": ["three", "two", "one"]
		}, {
			"protocol": "redis",
			"request": "ZREVRANGE myzset 2 3",
			"response": ["one"]
		}, {
			"protocol": "redis",
			"request": "ZREVRANGE myzset -2 -1",
			"response": ["two", "one"]
		}]
	}`,
}

var ZRevRangeByLex = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e", 0, "f", 0, "g")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(7))

		r2, err := c.ZRevRangeByLex(ctx, "myzset", "[c", "-")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, []string{"c", "b", "a"})

		r3, err := c.ZRevRangeByLex(ctx, "myzset", "(c", "-")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"b", "a"})

		r4, err := c.ZRevRangeByLex(ctx, "myzset", "(g", "[aaa")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"f", "e", "d", "c", "b"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g",
			"response": 7
		}, {
			"protocol": "redis",
			"request": "ZREVRANGEBYLEX myzset [c -",
			"response": ["c", "b", "a"]
		}, {
			"protocol": "redis",
			"request": "ZREVRANGEBYLEX myzset (c -",
			"response": ["b", "a"]
		}, {
			"protocol": "redis",
			"request": "ZREVRANGEBYLEX myzset (g [aaa",
			"response": ["f", "e", "d", "c", "b"]
		}]
	}`,
}

var ZRevRangeByScore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r4, []string{"three", "two", "one"})

		r5, err := c.ZRevRangeByScore(ctx, "myzset", "2", "1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"two", "one"})

		r6, err := c.ZRevRangeByScore(ctx, "myzset", "2", "(1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two"})

		r7, err := c.ZRevRangeByScore(ctx, "myzset", "(2", "(1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(r7), 0)
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZREVRANGEBYSCORE myzset +inf -inf",
			"response": ["three", "two", "one"]
		}, {
			"protocol": "redis",
			"request": "ZREVRANGEBYSCORE myzset 2 1",
			"response": ["two", "one"]
		}, {
			"protocol": "redis",
			"request": "ZREVRANGEBYSCORE myzset 2 (1",
			"response": ["two"]
		}, {
			"protocol": "redis",
			"request": "ZREVRANGEBYSCORE myzset (2 (1",
			"response": []
		}]
	}`,
}

var ZRevRank = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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

		_, err = c.ZRevRank(ctx, "myzset", "four")
		assert.True(t, redis.IsErrNil(err))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD myzset 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZREVRANK myzset one",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "ZREVRANK myzset four",
			"response": "(nil)"
		}]
	}`,
}

var ZScore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD myzset 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZSCORE myzset one",
			"response": "1"
		}]
	}`,
}

var ZUnion = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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
		assert.Equal(t, r6, []string{"one", "three", "two"})

		r7, err := c.ZUnionWithScores(ctx, 2, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 2}, {"three", 3}, {"two", 4}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD zset1 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset1 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZUNION 2 zset1 zset2",
			"response": ["one", "three", "two"]
		}, {
			"protocol": "redis",
			"request": "ZUNION 2 zset1 zset2 WITHSCORES",
			"response": ["one", "2", "three", "3", "two", "4"]
		}]
	}`,
}

var ZUnionStore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

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

		r7, err := c.ZRangeWithScores(ctx, "out", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 5}, {"three", 9}, {"two", 10}})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "ZADD zset1 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset1 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 1 one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 2 two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZADD zset2 3 three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "ZUNIONSTORE out 2 zset1 zset2 WEIGHTS 2 3",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "ZRANGE out 0 -1 WITHSCORES",
			"response": ["one", "5", "three", "9", "two", "10"]
		}]
	}`,
}
