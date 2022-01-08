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

var Del = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "key1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Set(ctx, "key2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, redis.OK)

		r3, err := c.Del(ctx, "key1", "key2", "key3")
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
			"request": "SET key1 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SET key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "DEL key1 key2 key3",
			"response": 2
		}]
	}`,
}

var Dump = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", 10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Dump(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "\u0000\xC0\n\t\u0000\xBEm\u0006\x89Z(\u0000\n")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey 10",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "DUMP mykey",
			"response": "@\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\"@"
		}]
	}`,
}

var Exists = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "key1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Exists(ctx, "key1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.Exists(ctx, "nosuchkey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))

		r4, err := c.Set(ctx, "key2", "World")
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, r4, redis.OK)

		r5, err := c.Exists(ctx, "key1", "key2", "nosuchkey")
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
			"request": "SET key1 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXISTS key1",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "EXISTS nosuchkey",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "SET key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXISTS key1 key2 nosuchkey",
			"response": 2
		}]
	}`,
}

var Expire = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Expire(ctx, "mykey", 10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(10))

		r4, err := c.Set(ctx, "mykey", "Hello World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, redis.OK)

		r5, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(-1))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXPIRE mykey 10",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}, {
			"protocol": "redis",
			"request": "SET mykey @\"Hello World\"@",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": -1
		}]
	}`,
}

var ExpireAt = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Exists(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.ExpireAt(ctx, "mykey", 1293840000)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 1)

		r4, err := c.Exists(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(0))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXISTS mykey",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "EXPIREAT mykey 1293840000",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "EXISTS mykey",
			"response": 0
		}]
	}`,
}

var Keys = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.MSet(ctx, "firstname", "Jack", "lastname", "Stuntman", "age", 35)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Keys(ctx, "*name*")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r2)
		assert.Equal(t, r2, []string{"firstname", "lastname"})

		r3, err := c.Keys(ctx, "a??")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"age"})

		r4, err := c.Keys(ctx, "*")
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(r4)
		assert.Equal(t, r4, []string{"age", "firstname", "lastname"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "MSET firstname Jack lastname Stuntman age 35",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "KEYS *name*",
			"response": ["lastname", "firstname"]
		}, {
			"protocol": "redis",
			"request": "KEYS a??",
			"response": ["age"]
		}, {
			"protocol": "redis",
			"request": "KEYS *",
			"response": ["age", "lastname", "firstname"]
		}]
	}`,
}

var Persist = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Expire(ctx, "mykey", 10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(10))

		r4, err := c.Persist(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, 1)

		r5, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(-1))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXPIRE mykey 10",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}, {
			"protocol": "redis",
			"request": "PERSIST mykey",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": -1
		}]
	}`,
}

var PExpire = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.PExpire(ctx, "mykey", 1500)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, r3 >= 1 && r3 <= 2)

		r4, err := c.PTTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, r4 >= 1400 && r4 <= 1500)
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "PEXPIRE mykey 1500",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "PTTL mykey",
			"response": 1499
		}]
	}`,
}

var PExpireAt = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.PExpireAt(ctx, "mykey", 1555555555005)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(-2))

		r4, err := c.PTTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(-2))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "PEXPIREAT mykey 1555555555005",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": -2
		}, {
			"protocol": "redis",
			"request": "PTTL mykey",
			"response": -2
		}]
	}`,
}

var PTTL = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Expire(ctx, "mykey", 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.PTTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, r3 >= 990 && r3 <= 1000)
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXPIRE mykey 1",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "PTTL mykey",
			"response": 1000
		}]
	}`,
}

var Rename = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Rename(ctx, "mykey", "myotherkey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, redis.OK)

		r3, err := c.Get(ctx, "myotherkey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "RENAME mykey myotherkey",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GET myotherkey",
			"response": "Hello"
		}]
	}`,
}

var RenameNX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Set(ctx, "myotherkey", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, redis.OK)

		r3, err := c.RenameNX(ctx, "mykey", "myotherkey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 0)

		r4, err := c.Get(ctx, "myotherkey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, "World")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SET myotherkey World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "RENAMENX mykey myotherkey",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "GET myotherkey",
			"response": "World"
		}]
	}`,
}

var Touch = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "key1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Set(ctx, "key2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, redis.OK)

		r3, err := c.Touch(ctx, "key1", "key2")
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
			"request": "SET key1 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SET key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "TOUCH key1 key2",
			"response": 2
		}]
	}`,
}

var TTL = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.Expire(ctx, "mykey", 10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(10))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXPIRE mykey 10",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}]
	}`,
}

var Type = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "key1", "value")
		if err != nil {
			return
		}
		assert.Equal(t, r1, redis.OK)

		r2, err := c.LPush(ctx, "key2", "value")
		if err != nil {
			return
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.SAdd(ctx, "key3", "value")
		if err != nil {
			return
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.Type(ctx, "key1")
		if err != nil {
			return
		}
		assert.Equal(t, r4, "string")

		r5, err := c.Type(ctx, "key2")
		if err != nil {
			return
		}
		assert.Equal(t, r5, "list")

		r6, err := c.Type(ctx, "key3")
		if err != nil {
			return
		}
		assert.Equal(t, r6, "set")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET key1 value",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "LPUSH key2 value",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key3 value",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TYPE key1",
			"response": "string"
		}, {
			"protocol": "redis",
			"request": "TYPE key2",
			"response": "list"
		}, {
			"protocol": "redis",
			"request": "TYPE key3",
			"response": "set"
		}]
	}`,
}
