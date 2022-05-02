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
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

var SAdd = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))

		r4, err := c.OpsForSet().SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r4)
		assert.Equal(t, r4, []string{"Hello", "World"})
	},
	Skip: true,
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD myset Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset World",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS myset",
			"Response": "\"Hello\",\"World\""
		}]
	}`,
}

var SCard = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SCard(ctx, "myset")
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
			"Request": "SADD myset Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SCARD myset",
			"Response": "\"2\""
		}]
	}`,
}

var SDiff = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForSet().SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.OpsForSet().SDiff(ctx, "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r7)
		assert.Equal(t, r7, []string{"a", "b"})
	},
	Skip: true,
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD key1 a",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 b",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 d",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 e",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SDIFF key1 key2",
			"Response": "\"a\",\"b\""
		}]
	}`,
}

var SDiffStore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForSet().SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.OpsForSet().SDiffStore(ctx, "key", "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, int64(2))

		r8, err := c.OpsForSet().SMembers(ctx, "key")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r8)
		assert.Equal(t, r8, []string{"a", "b"})
	},
	Skip: true,
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD key1 a",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 b",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 d",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 e",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SDIFFSTORE key key1 key2",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS key",
			"Response": "\"a\",\"b\""
		}]
	}`,
}

var SInter = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForSet().SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.OpsForSet().SInter(ctx, "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []string{"c"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD key1 a",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 b",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 d",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 e",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SINTER key1 key2",
			"Response": "\"c\""
		}]
	}`,
}

var SInterStore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForSet().SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.OpsForSet().SInterStore(ctx, "key", "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, int64(1))

		r8, err := c.OpsForSet().SMembers(ctx, "key")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r8, []string{"c"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD key1 a",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 b",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 d",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 e",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SINTERSTORE key key1 key2",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS key",
			"Response": "\"c\""
		}]
	}`,
}

var SIsMember = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SIsMember(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SIsMember(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))
	},
	Data: "",
}

var SMembers = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r3)
		assert.Equal(t, r3, []string{"Hello", "World"})
	},
	Skip: true,
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD myset Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS myset",
			"Response": "\"Hello\",\"World\""
		}]
	}`,
}

var SMIsMember = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(0))

		r3, err := c.OpsForSet().SMIsMember(ctx, "myset", "one", "notamember")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []int64{int64(1), int64(0)})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD myset one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset one",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMISMEMBER myset one notamember",
			"Response": "\"1\",\"0\""
		}]
	}`,
}

var SMove = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "myotherset", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SMove(ctx, "myset", "myotherset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one"})

		r6, err := c.OpsForSet().SMembers(ctx, "myotherset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r6)
		assert.Equal(t, r6, []string{"three", "two"})
	},
	Skip: true,
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD myset one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myotherset three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMOVE myset myotherset two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS myset",
			"Response": "\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS myotherset",
			"Response": "\"two\",\"three\""
		}]
	}`,
}

var SPop = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "myset", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SPop(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}

		r5, err := c.OpsForSet().SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}

		r6 := append([]string{r4}, r5...)
		sort.Strings(r6)
		assert.Equal(t, r6, []string{"one", "three", "two"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD myset one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SPOP myset",
			"Response": "\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS myset",
			"Response": "\"three\",\"one\""
		}]
	}`,
}

var SRandMember = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "one", "two", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(3))

		_, err = c.OpsForSet().SRandMember(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}

		r3, err := c.OpsForSet().SRandMemberN(ctx, "myset", 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(r3), 2)

		r4, err := c.OpsForSet().SRandMemberN(ctx, "myset", -5)
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
			"Request": "SADD myset one two three",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "SRANDMEMBER myset",
			"Response": "\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "SRANDMEMBER myset 2",
			"Response": "\"one\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "SRANDMEMBER myset -5",
			"Response": "\"one\",\"one\",\"one\",\"two\",\"one\""
		}]
	}`,
}

var SRem = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "myset", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SRem(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SRem(ctx, "myset", "four")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(0))

		r6, err := c.OpsForSet().SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r6)
		assert.Equal(t, r6, []string{"three", "two"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD myset one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset two",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD myset three",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SREM myset one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SREM myset four",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS myset",
			"Response": "\"three\",\"two\""
		}]
	}`,
}

var SUnion = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForSet().SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.OpsForSet().SUnion(ctx, "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r7)
		assert.Equal(t, r7, []string{"a", "b", "c", "d", "e"})
	},
	Skip: true,
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD key1 a",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 b",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 d",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 e",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SUNION key1 key2",
			"Response": "\"a\",\"b\",\"c\",\"d\",\"e\""
		}]
	}`,
}

var SUnionStore = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForSet().SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForSet().SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.OpsForSet().SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.OpsForSet().SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.OpsForSet().SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.OpsForSet().SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.OpsForSet().SUnionStore(ctx, "key", "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, int64(5))

		r8, err := c.OpsForSet().SMembers(ctx, "key")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r8)
		assert.Equal(t, r8, []string{"a", "b", "c", "d", "e"})
	},
	Skip: true,
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SADD key1 a",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 b",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key1 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 c",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 d",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SADD key2 e",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SUNIONSTORE key key1 key2",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "SMEMBERS key",
			"Response": "\"a\",\"b\",\"c\",\"d\",\"e\""
		}]
	}`,
}
