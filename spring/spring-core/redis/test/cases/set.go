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
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))

		r4, err := c.SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r4)
		assert.Equal(t, r4, []string{"Hello", "World"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD myset Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset World",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "SMEMBERS myset",
			"response": ["Hello", "World"]
		}]
	}`,
}

var SCard = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SCard(ctx, "myset")
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
			"request": "SADD myset Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SCARD myset",
			"response": 2
		}]
	}`,
}

var SDiff = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.SDiff(ctx, "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r7)
		assert.Equal(t, r7, []string{"a", "b"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD key1 a",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 b",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 d",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 e",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SDIFF key1 key2",
			"response": ["a", "b"]
		}]
	}`,
}

var SDiffStore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.SDiffStore(ctx, "key", "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, int64(2))

		r8, err := c.SMembers(ctx, "key")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r8)
		assert.Equal(t, r8, []string{"a", "b"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD key1 a",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 b",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 d",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 e",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SDIFFSTORE key key1 key2",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "SMEMBERS key",
			"response": ["a", "b"]
		}]
	}`,
}

var SInter = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.SInter(ctx, "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []string{"c"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD key1 a",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 b",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 d",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 e",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SINTER key1 key2",
			"response": ["c"]
		}]
	}`,
}

var SInterStore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.SInterStore(ctx, "key", "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, int64(1))

		r8, err := c.SMembers(ctx, "key")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r8, []string{"c"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD key1 a",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 b",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 d",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 e",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SINTERSTORE key key1 key2",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SMEMBERS key",
			"response": ["c"]
		}]
	}`,
}

var SIsMember = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SIsMember(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SIsMember(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))
	},
	Data: "",
}

var SMembers = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "myset", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r3)
		assert.Equal(t, r3, []string{"Hello", "World"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD myset Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SMEMBERS myset",
			"response": ["Hello", "World"]
		}]
	}`,
}

var SMIsMember = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(0))

		r3, err := c.SMIsMember(ctx, "myset", "one", "notamember")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []int64{int64(1), int64(0)})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD myset one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset one",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "SMISMEMBER myset one notamember",
			"response": [1, 0]
		}]
	}`,
}

var SMove = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "myotherset", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SMove(ctx, "myset", "myotherset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, 1)

		r5, err := c.SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one"})

		r6, err := c.SMembers(ctx, "myotherset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r6)
		assert.Equal(t, r6, []string{"three", "two"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD myset one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myotherset three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SMOVE myset myotherset two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SMEMBERS myset",
			"response": ["one"]
		}, {
			"protocol": "redis",
			"request": "SMEMBERS myotherset",
			"response": ["three", "two"]
		}]
	}`,
}

var SPop = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "myset", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SPop(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}

		r5, err := c.SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}

		r6 := append([]string{r4}, r5...)
		sort.Strings(r6)
		assert.Equal(t, r6, []string{"one", "three", "two"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD myset one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SPOP myset",
			"response": "two"
		}, {
			"protocol": "redis",
			"request": "SMEMBERS myset",
			"response": ["three", "one"]
		}]
	}`,
}

var SRandMember = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "one", "two", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(3))

		_, err = c.SRandMember(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}

		r3, err := c.SRandMemberN(ctx, "myset", 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, len(r3), 2)

		r4, err := c.SRandMemberN(ctx, "myset", -5)
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
			"request": "SADD myset one two three",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "SRANDMEMBER myset",
			"response": "one"
		}, {
			"protocol": "redis",
			"request": "SRANDMEMBER myset 2",
			"response": ["one", "three"]
		}, {
			"protocol": "redis",
			"request": "SRANDMEMBER myset -5",
			"response": ["one", "one", "one", "two", "one"]
		}]
	}`,
}

var SRem = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "myset", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "myset", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SRem(ctx, "myset", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.SRem(ctx, "myset", "four")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(0))

		r6, err := c.SMembers(ctx, "myset")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r6)
		assert.Equal(t, r6, []string{"three", "two"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD myset one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset two",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD myset three",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SREM myset one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SREM myset four",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "SMEMBERS myset",
			"response": ["three", "two"]
		}]
	}`,
}

var SUnion = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.SUnion(ctx, "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r7)
		assert.Equal(t, r7, []string{"a", "b", "c", "d", "e"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD key1 a",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 b",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 d",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 e",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SUNION key1 key2",
			"response": ["a", "b", "c", "e", "d"]
		}]
	}`,
}

var SUnionStore = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SAdd(ctx, "key1", "a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.SAdd(ctx, "key1", "b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "key1", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.SAdd(ctx, "key2", "c")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(1))

		r5, err := c.SAdd(ctx, "key2", "d")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(1))

		r6, err := c.SAdd(ctx, "key2", "e")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, int64(1))

		r7, err := c.SUnionStore(ctx, "key", "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, int64(5))

		r8, err := c.SMembers(ctx, "key")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r8)
		assert.Equal(t, r8, []string{"a", "b", "c", "d", "e"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SADD key1 a",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 b",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key1 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 c",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 d",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key2 e",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SUNIONSTORE key key1 key2",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "SMEMBERS key",
			"response": ["a", "b", "c", "e", "d"]
		}]
	}`,
}
