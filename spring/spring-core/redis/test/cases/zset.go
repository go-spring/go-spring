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
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "uno")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(2))

		r4, err := c.OpsForZSet().ZRangeWithScores(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []redis.ZItem{{"one", 1}, {"uno", 1}, {"two", 2}, {"three", 3}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 uno",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two 3 three",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1 WITHSCORES",
			"Response": "\"one\",\"1\",\"uno\",\"1\",\"two\",\"2\",\"three\",\"3\""
		}]
	}`,
}

var ZCard = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZCard(ctx, "myzset")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(2))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZCARD myzset",
			"Response": "\"2\""
		}]
	}`,
}

var ZCount = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZCount(ctx, "myzset", "-inf", "+inf")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(3))

		r5, err := c.OpsForZSet().ZCount(ctx, "myzset", "(1", "3")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(2))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZCOUNT myzset -inf +inf",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZCOUNT myzset (1 3",
			"Response": "\"2\""
		}]
	}`,
}

var ZDiff = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "zset1", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "zset1", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "zset1", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZAdd(ctx, "zset2", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForZSet().ZAdd(ctx, "zset2", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForZSet().ZDiff(ctx, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"three"})

		r7, err := c.OpsForZSet().ZDiffWithScores(ctx, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"three", 3}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD zset1 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset1 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset1 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZDIFF 2 zset1 zset2",
			"Response": "\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZDIFF 2 zset1 zset2 WITHSCORES",
			"Response": "\"three\",\"3\""
		}]
	}`,
}

var ZIncrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZIncrBy(ctx, "myzset", 2, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, float64(3))

		r4, err := c.OpsForZSet().ZRangeWithScores(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []redis.ZItem{{"two", 2}, {"one", 3}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZINCRBY myzset 2 one",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1 WITHSCORES",
			"Response": "\"two\",\"2\",\"one\",\"3\""
		}]
	}`,
}

var ZInter = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "zset1", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "zset1", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "zset2", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZAdd(ctx, "zset2", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForZSet().ZAdd(ctx, "zset2", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForZSet().ZInter(ctx, 2, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"one", "two"})

		r7, err := c.OpsForZSet().ZInterWithScores(ctx, 2, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 2}, {"two", 4}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD zset1 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset1 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZINTER 2 zset1 zset2",
			"Response": "\"one\",\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZINTER 2 zset1 zset2 WITHSCORES",
			"Response": "\"one\",\"2\",\"two\",\"4\""
		}]
	}`,
}

var ZLexCount = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(5))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 0, "f", 0, "g")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.OpsForZSet().ZLexCount(ctx, "myzset", "-", "+")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(7))

		r4, err := c.OpsForZSet().ZLexCount(ctx, "myzset", "[b", "[f")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(5))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 0 f 0 g",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZLEXCOUNT myzset - +",
			"Response": "\"7\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZLEXCOUNT myzset [b [f",
			"Response": "\"5\""
		}]
	}`,
}

var ZMScore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZMScore(ctx, "myzset", "one", "two", "nofield")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []float64{1, 2, 0})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZMSCORE myzset one two nofield",
			"Response": "\"1\",\"2\",NULL"
		}]
	}`,
}

var ZPopMax = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZPopMax(ctx, "myzset")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []redis.ZItem{{"three", 3}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZPOPMAX myzset",
			"Response": "\"three\",\"3\""
		}]
	}`,
}

var ZPopMin = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZPopMin(ctx, "myzset")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []redis.ZItem{{"one", 1}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZPOPMIN myzset",
			"Response": "\"one\",\"1\""
		}]
	}`,
}

var ZRandMember = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "dadi", 1, "uno", 2, "due", 3, "tre", 4, "quattro", 5, "cinque", 6, "sei")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(6))

		r2, err := c.OpsForZSet().ZRandMember(ctx, "dadi")
		if err != nil {
			t.Fatal(err)
		}
		assert.NotEqual(t, r2, "")

		r3, err := c.OpsForZSet().ZRandMember(ctx, "dadi")
		if err != nil {
			t.Fatal(err)
		}
		assert.NotEqual(t, r3, "")

		r4, err := c.OpsForZSet().ZRandMemberWithScores(ctx, "dadi", -5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(r4), 5)
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD dadi 1 uno 2 due 3 tre 4 quattro 5 cinque 6 sei",
			"Response": "\"6\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANDMEMBER dadi",
			"Response": "\"sei\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANDMEMBER dadi",
			"Response": "\"sei\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANDMEMBER dadi -5 WITHSCORES",
			"Response": "\"uno\",\"1\",\"uno\",\"1\",\"cinque\",\"5\",\"sei\",\"6\",\"due\",\"2\""
		}]
	}`,
}

var ZRange = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRange(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"one", "two", "three"})

		r5, err := c.OpsForZSet().ZRange(ctx, "myzset", 2, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"three"})

		r6, err := c.OpsForZSet().ZRange(ctx, "myzset", -2, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two", "three"})

		r7, err := c.OpsForZSet().ZRangeWithScores(ctx, "myzset", 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 1}, {"two", 2}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1",
			"Response": "\"one\",\"two\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 2 3",
			"Response": "\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset -2 -1",
			"Response": "\"two\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 1 WITHSCORES",
			"Response": "\"one\",\"1\",\"two\",\"2\""
		}]
	}`,
}

var ZRangeByLex = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e", 0, "f", 0, "g")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(7))

		r2, err := c.OpsForZSet().ZRangeByLex(ctx, "myzset", "-", "[c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, []string{"a", "b", "c"})

		r3, err := c.OpsForZSet().ZRangeByLex(ctx, "myzset", "-", "(c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"a", "b"})

		r4, err := c.OpsForZSet().ZRangeByLex(ctx, "myzset", "[aaa", "(g")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"b", "c", "d", "e", "f"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g",
			"Response": "\"7\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGEBYLEX myzset - [c",
			"Response": "\"a\",\"b\",\"c\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGEBYLEX myzset - (c",
			"Response": "\"a\",\"b\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGEBYLEX myzset [aaa (g",
			"Response": "\"b\",\"c\",\"d\",\"e\",\"f\""
		}]
	}`,
}

var ZRangeByScore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRangeByScore(ctx, "myzset", "-inf", "+inf")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"one", "two", "three"})

		r5, err := c.OpsForZSet().ZRangeByScore(ctx, "myzset", "1", "2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one", "two"})

		r6, err := c.OpsForZSet().ZRangeByScore(ctx, "myzset", "(1", "2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two"})

		r7, err := c.OpsForZSet().ZRangeByScore(ctx, "myzset", "(1", "(2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(r7), 0)
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGEBYSCORE myzset -inf +inf",
			"Response": "\"one\",\"two\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGEBYSCORE myzset 1 2",
			"Response": "\"one\",\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGEBYSCORE myzset (1 2",
			"Response": "\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGEBYSCORE myzset (1 (2",
			"Response": ""
		}]
	}`,
}

var ZRank = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRank(ctx, "myzset", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(2))

		_, err = c.OpsForZSet().ZRank(ctx, "myzset", "four")
		assert.True(t, redis.IsErrNil(err))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANK myzset three",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANK myzset four",
			"Response": "NULL"
		}]
	}`,
}

var ZRem = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRem(ctx, "myzset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForZSet().ZRangeWithScores(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []redis.ZItem{{"one", 1}, {"three", 3}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREM myzset two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1 WITHSCORES",
			"Response": "\"one\",\"1\",\"three\",\"3\""
		}]
	}`,
}

var ZRemRangeByLex = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 0, "aaaa", 0, "b", 0, "c", 0, "d", 0, "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(5))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 0, "foo", 0, "zap", 0, "zip", 0, "ALPHA", 0, "alpha")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(5))

		r3, err := c.OpsForZSet().ZRange(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{
			"ALPHA", "aaaa", "alpha", "b", "c", "d", "e", "foo", "zap", "zip",
		})

		r4, err := c.OpsForZSet().ZRemRangeByLex(ctx, "myzset", "[alpha", "[omega")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(6))

		r5, err := c.OpsForZSet().ZRange(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"ALPHA", "aaaa", "zap", "zip"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 0 aaaa 0 b 0 c 0 d 0 e",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 0 foo 0 zap 0 zip 0 ALPHA 0 alpha",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1",
			"Response": "\"ALPHA\",\"aaaa\",\"alpha\",\"b\",\"c\",\"d\",\"e\",\"foo\",\"zap\",\"zip\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREMRANGEBYLEX myzset [alpha [omega",
			"Response": "\"6\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1",
			"Response": "\"ALPHA\",\"aaaa\",\"zap\",\"zip\""
		}]
	}`,
}

var ZRemRangeByRank = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRemRangeByRank(ctx, "myzset", 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(2))

		r5, err := c.OpsForZSet().ZRangeWithScores(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []redis.ZItem{{"three", 3}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREMRANGEBYRANK myzset 0 1",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1 WITHSCORES",
			"Response": "\"three\",\"3\""
		}]
	}`,
}

var ZRemRangeByScore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRemRangeByScore(ctx, "myzset", "-inf", "(2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForZSet().ZRangeWithScores(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []redis.ZItem{{"two", 2}, {"three", 3}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREMRANGEBYSCORE myzset -inf (2",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE myzset 0 -1 WITHSCORES",
			"Response": "\"two\",\"2\",\"three\",\"3\""
		}]
	}`,
}

var ZRevRange = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRevRange(ctx, "myzset", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"three", "two", "one"})

		r5, err := c.OpsForZSet().ZRevRange(ctx, "myzset", 2, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one"})

		r6, err := c.OpsForZSet().ZRevRange(ctx, "myzset", -2, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two", "one"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGE myzset 0 -1",
			"Response": "\"three\",\"two\",\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGE myzset 2 3",
			"Response": "\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGE myzset -2 -1",
			"Response": "\"two\",\"one\""
		}]
	}`,
}

var ZRevRangeByLex = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 0, "a", 0, "b", 0, "c", 0, "d", 0, "e", 0, "f", 0, "g")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(7))

		r2, err := c.OpsForZSet().ZRevRangeByLex(ctx, "myzset", "[c", "-")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, []string{"c", "b", "a"})

		r3, err := c.OpsForZSet().ZRevRangeByLex(ctx, "myzset", "(c", "-")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"b", "a"})

		r4, err := c.OpsForZSet().ZRevRangeByLex(ctx, "myzset", "(g", "[aaa")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"f", "e", "d", "c", "b"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g",
			"Response": "\"7\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGEBYLEX myzset [c -",
			"Response": "\"c\",\"b\",\"a\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGEBYLEX myzset (c -",
			"Response": "\"b\",\"a\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGEBYLEX myzset (g [aaa",
			"Response": "\"f\",\"e\",\"d\",\"c\",\"b\""
		}]
	}`,
}

var ZRevRangeByScore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRevRangeByScore(ctx, "myzset", "+inf", "-inf")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"three", "two", "one"})

		r5, err := c.OpsForZSet().ZRevRangeByScore(ctx, "myzset", "2", "1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"two", "one"})

		r6, err := c.OpsForZSet().ZRevRangeByScore(ctx, "myzset", "2", "(1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two"})

		r7, err := c.OpsForZSet().ZRevRangeByScore(ctx, "myzset", "(2", "(1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(r7), 0)
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGEBYSCORE myzset +inf -inf",
			"Response": "\"three\",\"two\",\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGEBYSCORE myzset 2 1",
			"Response": "\"two\",\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGEBYSCORE myzset 2 (1",
			"Response": "\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANGEBYSCORE myzset (2 (1",
			"Response": ""
		}]
	}`,
}

var ZRevRank = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "myzset", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "myzset", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZRevRank(ctx, "myzset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(2))

		_, err = c.OpsForZSet().ZRevRank(ctx, "myzset", "four")
		assert.True(t, redis.IsErrNil(err))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD myzset 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANK myzset one",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZREVRANK myzset four",
			"Response": "NULL"
		}]
	}`,
}

var ZScore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "myzset", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZScore(ctx, "myzset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, float64(1))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD myzset 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZSCORE myzset one",
			"Response": "\"1\""
		}]
	}`,
}

var ZUnion = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "zset1", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "zset1", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "zset2", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZAdd(ctx, "zset2", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForZSet().ZAdd(ctx, "zset2", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForZSet().ZUnion(ctx, 2, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"one", "three", "two"})

		r7, err := c.OpsForZSet().ZUnionWithScores(ctx, 2, "zset1", "zset2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 2}, {"three", 3}, {"two", 4}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD zset1 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset1 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZUNION 2 zset1 zset2",
			"Response": "\"one\",\"three\",\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZUNION 2 zset1 zset2 WITHSCORES",
			"Response": "\"one\",\"2\",\"three\",\"3\",\"two\",\"4\""
		}]
	}`,
}

var ZUnionStore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForZSet().ZAdd(ctx, "zset1", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForZSet().ZAdd(ctx, "zset1", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForZSet().ZAdd(ctx, "zset2", 1, "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForZSet().ZAdd(ctx, "zset2", 2, "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForZSet().ZAdd(ctx, "zset2", 3, "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForZSet().ZUnionStore(ctx, "out", 2, "zset1", "zset2", "WEIGHTS", 2, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(3))

		r7, err := c.OpsForZSet().ZRangeWithScores(ctx, "out", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []redis.ZItem{{"one", 5}, {"three", 9}, {"two", 10}})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "ZADD zset1 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset1 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 1 one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 2 two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZADD zset2 3 three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZUNIONSTORE out 2 zset1 zset2 WEIGHTS 2 3",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "ZRANGE out 0 -1 WITHSCORES",
			"Response": "\"one\",\"5\",\"three\",\"9\",\"two\",\"10\""
		}]
	}`,
}
