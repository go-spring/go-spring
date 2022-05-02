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

var Append = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForKey().Exists(ctx, "mykey")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r1, int64(0))

		r2, err := c.OpsForString().Append(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r2, int64(5))

		r3, err := c.OpsForString().Append(ctx, "mykey", " World")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r3, int64(11))

		r4, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r4, "Hello World")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "EXISTS mykey",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "APPEND mykey Hello",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "APPEND mykey \" World\"",
			"Response": "\"11\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"Hello World\""
		}]
	}`,
}

var Decr = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().Decr(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(9))

		r3, err := c.OpsForString().Set(ctx, "mykey", "234293482390480948029348230948")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r3))

		_, err = c.OpsForString().Decr(ctx, "mykey")
		assert.Error(t, err, "ERR value is not an integer or out of range")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey 10",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "DECR mykey",
			"Response": "\"9\""
		}, {
			"Protocol": "REDIS",
			"Request": "SET mykey 234293482390480948029348230948",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "DECR mykey",
			"Response": "(err) ERR value is not an integer or out of range"
		}]
	}`,
}

var DecrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().DecrBy(ctx, "mykey", 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(7))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey 10",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "DECRBY mykey 3",
			"Response": "\"7\""
		}]
	}`,
}

var Get = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		_, err := c.OpsForString().Get(ctx, "nonexisting")
		assert.True(t, redis.IsErrNil(err))

		r2, err := c.OpsForString().Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r2))

		r3, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "GET nonexisting",
			"Response": "NULL"
		}, {
			"Protocol": "REDIS",
			"Request": "SET mykey Hello",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"Hello\""
		}]
	}`,
}

var GetDel = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().GetDel(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")

		_, err = c.OpsForString().Get(ctx, "mykey")
		assert.True(t, redis.IsErrNil(err))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey Hello",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETDEL mykey",
			"Response": "\"Hello\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "NULL"
		}]
	}`,
}

var GetRange = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "This is a string")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().GetRange(ctx, "mykey", 0, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "This")

		r3, err := c.OpsForString().GetRange(ctx, "mykey", -3, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "ing")

		r4, err := c.OpsForString().GetRange(ctx, "mykey", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, "This is a string")

		r5, err := c.OpsForString().GetRange(ctx, "mykey", 10, 100)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, "string")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey \"This is a string\"",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETRANGE mykey 0 3",
			"Response": "\"This\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETRANGE mykey -3 -1",
			"Response": "\"ing\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETRANGE mykey 0 -1",
			"Response": "\"This is a string\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETRANGE mykey 10 100",
			"Response": "\"string\""
		}]
	}`,
}

var GetSet = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Incr(ctx, "mycounter")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForString().GetSet(ctx, "mycounter", "0")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "1")

		r3, err := c.OpsForString().Get(ctx, "mycounter")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "0")

		r4, err := c.OpsForString().Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r4))

		r5, err := c.OpsForString().GetSet(ctx, "mykey", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, "Hello")

		r6, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, "World")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "INCR mycounter",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETSET mycounter 0",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mycounter",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "SET mykey Hello",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETSET mykey World",
			"Response": "\"Hello\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"World\""
		}]
	}`,
}

var Incr = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().Incr(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(11))

		r3, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "11")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey 10",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "INCR mykey",
			"Response": "\"11\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"11\""
		}]
	}`,
}

var IncrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().IncrBy(ctx, "mykey", 5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(15))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey 10",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "INCRBY mykey 5",
			"Response": "\"15\""
		}]
	}`,
}

var IncrByFloat = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", 10.50)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().IncrByFloat(ctx, "mykey", 0.1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 10.6)

		r3, err := c.OpsForString().IncrByFloat(ctx, "mykey", -5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 5.6)

		r4, err := c.OpsForString().Set(ctx, "mykey", 5.0e3)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r4))

		r5, err := c.OpsForString().IncrByFloat(ctx, "mykey", 2.0e2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, float64(5200))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey 10.5",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "INCRBYFLOAT mykey 0.1",
			"Response": "\"10.6\""
		}, {
			"Protocol": "REDIS",
			"Request": "INCRBYFLOAT mykey -5",
			"Response": "\"5.6\""
		}, {
			"Protocol": "REDIS",
			"Request": "SET mykey 5000",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "INCRBYFLOAT mykey 200",
			"Response": "\"5200\""
		}]
	}`,
}

var MGet = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "key1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().Set(ctx, "key2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r2))

		r3, err := c.OpsForString().MGet(ctx, "key1", "key2", "nonexisting")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []interface{}{"Hello", "World", nil})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET key1 Hello",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "SET key2 World",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "MGET key1 key2 nonexisting",
			"Response": "\"Hello\",\"World\",NULL"
		}]
	}`,
}

var MSet = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().MSet(ctx, "key1", "Hello", "key2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().Get(ctx, "key1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")

		r3, err := c.OpsForString().Get(ctx, "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "World")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "MSET key1 Hello key2 World",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET key1",
			"Response": "\"Hello\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET key2",
			"Response": "\"World\""
		}]
	}`,
}

var MSetNX = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().MSetNX(ctx, "key1", "Hello", "key2", "there")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForString().MSetNX(ctx, "key2", "new", "key3", "world")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(0))

		r3, err := c.OpsForString().MGet(ctx, "key1", "key2", "key3")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []interface{}{"Hello", "there", nil})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "MSETNX key1 Hello key2 there",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "MSETNX key2 new key3 world",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "MGET key1 key2 key3",
			"Response": "\"Hello\",\"there\",NULL"
		}]
	}`,
}

var PSetEX = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().PSetEX(ctx, "mykey", "Hello", 1000)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForKey().PTTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, r2 <= 1000 && r2 >= 900)

		r3, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "PSETEX mykey 1000 Hello",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "PTTL mykey",
			"Response": "\"1000\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"Hello\""
		}]
	}`,
}

var Set = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")

		r3, err := c.OpsForString().SetEX(ctx, "anotherkey", "will expire in a minute", 60)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r3))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey Hello",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"Hello\""
		}, {
			"Protocol": "REDIS",
			"Request": "SETEX anotherkey 60 \"will expire in a minute\"",
			"Response": "\"OK\""
		}]
	}`,
}

var SetEX = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().SetEX(ctx, "mykey", "Hello", 10)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForKey().TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(10))

		r3, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SETEX mykey 10 Hello",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "TTL mykey",
			"Response": "\"10\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"Hello\""
		}]
	}`,
}

var SetNX = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().SetNX(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.OpsForString().SetNX(ctx, "mykey", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(0))

		r3, err := c.OpsForString().Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SETNX mykey Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "SETNX mykey World",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"Hello\""
		}]
	}`,
}

var SetRange = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "key1", "Hello World")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().SetRange(ctx, "key1", 6, "Redis")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(11))

		r3, err := c.OpsForString().Get(ctx, "key1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello Redis")

		r4, err := c.OpsForString().SetRange(ctx, "key2", 6, "Redis")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(11))

		r5, err := c.OpsForString().Get(ctx, "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, "\u0000\u0000\u0000\u0000\u0000\u0000Redis")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET key1 \"Hello World\"",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "SETRANGE key1 6 Redis",
			"Response": "\"11\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET key1",
			"Response": "\"Hello Redis\""
		}, {
			"Protocol": "REDIS",
			"Request": "SETRANGE key2 6 Redis",
			"Response": "\"11\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET key2",
			"Response": "\"\\x00\\x00\\x00\\x00\\x00\\x00Redis\""
		}]
	}`,
}

var StrLen = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.OpsForString().Set(ctx, "mykey", "Hello world")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.OpsForString().StrLen(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(11))

		r3, err := c.OpsForString().StrLen(ctx, "nonexisting")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey \"Hello world\"",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "STRLEN mykey",
			"Response": "\"11\""
		}, {
			"Protocol": "REDIS",
			"Request": "STRLEN nonexisting",
			"Response": "\"0\""
		}]
	}`,
}
