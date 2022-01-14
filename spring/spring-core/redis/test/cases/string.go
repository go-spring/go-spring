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
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Exists(ctx, "mykey")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r1, int64(0))

		r2, err := c.Append(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r2, int64(5))

		r3, err := c.Append(ctx, "mykey", " World")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r3, int64(11))

		r4, err := c.Get(ctx, "mykey")
		if err != nil {
			t.Fatal()
		}
		assert.Equal(t, r4, "Hello World")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "EXISTS mykey",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "APPEND mykey Hello",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "APPEND mykey @\" World\"@",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello World"
		}]
	}`,
}

var Decr = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.Decr(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(9))

		r3, err := c.Set(ctx, "mykey", "234293482390480948029348230948")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r3))

		_, err = c.Decr(ctx, "mykey")
		assert.Error(t, err, "ERR value is not an integer or out of range")
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
			"request": "DECR mykey",
			"response": 9
		}, {
			"protocol": "redis",
			"request": "SET mykey 234293482390480948029348230948",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "DECR mykey",
			"response": "(err) ERR value is not an integer or out of range"
		}]
	}`,
}

var DecrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.DecrBy(ctx, "mykey", 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(7))
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
			"request": "DECRBY mykey 3",
			"response": 7
		}]
	}`,
}

var Get = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		_, err := c.Get(ctx, "nonexisting")
		assert.True(t, redis.IsErrNil(err))

		r2, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r2))

		r3, err := c.Get(ctx, "mykey")
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
			"request": "GET nonexisting",
			"response": "(nil)"
		}, {
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello"
		}]
	}`,
}

var GetDel = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.GetDel(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")

		_, err = c.Get(ctx, "mykey")
		assert.True(t, redis.IsErrNil(err))
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
			"request": "GETDEL mykey",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "(nil)"
		}]
	}`,
}

var GetRange = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "This is a string")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.GetRange(ctx, "mykey", 0, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "This")

		r3, err := c.GetRange(ctx, "mykey", -3, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "ing")

		r4, err := c.GetRange(ctx, "mykey", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, "This is a string")

		r5, err := c.GetRange(ctx, "mykey", 10, 100)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, "string")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey @\"This is a string\"@",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey 0 3",
			"response": "This"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey -3 -1",
			"response": "ing"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey 0 -1",
			"response": "This is a string"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey 10 100",
			"response": "string"
		}]
	}`,
}

var GetSet = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Incr(ctx, "mycounter")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.GetSet(ctx, "mycounter", "0")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "1")

		r3, err := c.Get(ctx, "mycounter")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "0")

		r4, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r4))

		r5, err := c.GetSet(ctx, "mykey", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, "Hello")

		r6, err := c.Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, "World")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "INCR mycounter",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "GETSET mycounter 0",
			"response": "1"
		}, {
			"protocol": "redis",
			"request": "GET mycounter",
			"response": "0"
		}, {
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GETSET mykey World",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "World"
		}]
	}`,
}

var Incr = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.Incr(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(11))

		r3, err := c.Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "11")
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
			"request": "INCR mykey",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "11"
		}]
	}`,
}

var IncrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "10")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.IncrBy(ctx, "mykey", 5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(15))
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
			"request": "INCRBY mykey 5",
			"response": 15
		}]
	}`,
}

var IncrByFloat = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", 10.50)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.IncrByFloat(ctx, "mykey", 0.1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 10.6)

		r3, err := c.IncrByFloat(ctx, "mykey", -5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 5.6)

		r4, err := c.Set(ctx, "mykey", 5.0e3)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r4))

		r5, err := c.IncrByFloat(ctx, "mykey", 2.0e2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, float64(5200))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey 10.5",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "INCRBYFLOAT mykey 0.1",
			"response": "10.6"
		}, {
			"protocol": "redis",
			"request": "INCRBYFLOAT mykey -5",
			"response": "5.6"
		}, {
			"protocol": "redis",
			"request": "SET mykey 5000",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "INCRBYFLOAT mykey 200",
			"response": "5200"
		}]
	}`,
}

var MGet = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "key1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.Set(ctx, "key2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r2))

		r3, err := c.MGet(ctx, "key1", "key2", "nonexisting")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []interface{}{"Hello", "World", nil})
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
			"request": "MGET key1 key2 nonexisting",
			"response": ["Hello", "World", null]
		}]
	}`,
}

var MSet = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.MSet(ctx, "key1", "Hello", "key2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.Get(ctx, "key1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")

		r3, err := c.Get(ctx, "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "World")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "MSET key1 Hello key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GET key1",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "GET key2",
			"response": "World"
		}]
	}`,
}

var MSetNX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.MSetNX(ctx, "key1", "Hello", "key2", "there")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, 1)

		r2, err := c.MSetNX(ctx, "key2", "new", "key3", "world")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 0)

		r3, err := c.MGet(ctx, "key1", "key2", "key3")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []interface{}{"Hello", "there", nil})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "MSETNX key1 Hello key2 there",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "MSETNX key2 new key3 world",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "MGET key1 key2 key3",
			"response": ["Hello", "there", null]
		}]
	}`,
}

var PSetEX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.PSetEX(ctx, "mykey", "Hello", 1000)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.PTTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, r2 <= 1000 && r2 >= 900)

		r3, err := c.Get(ctx, "mykey")
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
			"request": "PSETEX mykey 1000 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "PTTL mykey",
			"response": 1000
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello"
		}]
	}`,
}

var Set = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")

		r3, err := c.SetEX(ctx, "anotherkey", "will expire in a minute", 60)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r3))
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
			"request": "GET mykey",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "SETEX anotherkey 60 @\"will expire in a minute\"@",
			"response": "OK"
		}]
	}`,
}

var SetEX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SetEX(ctx, "mykey", "Hello", 10)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.TTL(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(10))

		r3, err := c.Get(ctx, "mykey")
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
			"request": "SETEX mykey 10 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello"
		}]
	}`,
}

var SetNX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SetNX(ctx, "mykey", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, 1)

		r2, err := c.SetNX(ctx, "mykey", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 0)

		r3, err := c.Get(ctx, "mykey")
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
			"request": "SETNX mykey Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SETNX mykey World",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello"
		}]
	}`,
}

var SetRange = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "key1", "Hello World")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.SetRange(ctx, "key1", 6, "Redis")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(11))

		r3, err := c.Get(ctx, "key1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello Redis")

		r4, err := c.SetRange(ctx, "key2", 6, "Redis")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(11))

		r5, err := c.Get(ctx, "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, "\u0000\u0000\u0000\u0000\u0000\u0000Redis")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET key1 @\"Hello World\"@",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SETRANGE key1 6 Redis",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET key1",
			"response": "Hello Redis"
		}, {
			"protocol": "redis",
			"request": "SETRANGE key2 6 Redis",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET key2",
			"response": "\u0000\u0000\u0000\u0000\u0000\u0000Redis"
		}]
	}`,
}

var StrLen = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "Hello world")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.OK(r1))

		r2, err := c.StrLen(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(11))

		r3, err := c.StrLen(ctx, "nonexisting")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey @\"Hello world\"@",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "STRLEN mykey",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "STRLEN nonexisting",
			"response": 0
		}]
	}`,
}
