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

var HDel = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "foo")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HDel(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HDel(ctx, "myhash", "field2")
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
			"request": "HSET myhash field1 foo",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HDEL myhash field1",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HDEL myhash field2",
			"response": 0
		}]
	}`,
}

var HExists = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "foo")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HExists(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.HExists(ctx, "myhash", "field2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 0)
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash field1 foo",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HEXISTS myhash field1",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HEXISTS myhash field2",
			"response": 0
		}]
	}`,
}

var HGet = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "foo")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HGet(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "foo")

		_, err = c.HGet(ctx, "myhash", "field2")
		assert.Equal(t, err, redis.ErrNil)
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash field1 foo",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HGET myhash field1",
			"response": "foo"
		}, {
			"protocol": "redis",
			"request": "HGET myhash field2",
			"response": "(nil)"
		}]
	}`,
}

var HGetAll = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HGetAll(ctx, "myhash")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, map[string]string{
			"field1": "Hello",
			"field2": "World",
		})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash field1 Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HSET myhash field2 World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HGETALL myhash",
			"response": {
				"field1": "Hello",
				"field2": "World"
			}
		}]
	}`,
}

var HIncrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field", 5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HIncrBy(ctx, "myhash", "field", 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(6))

		r3, err := c.HIncrBy(ctx, "myhash", "field", -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(5))

		r4, err := c.HIncrBy(ctx, "myhash", "field", -10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(-5))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash field 5",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HINCRBY myhash field 1",
			"response": 6
		}, {
			"protocol": "redis",
			"request": "HINCRBY myhash field -1",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "HINCRBY myhash field -10",
			"response": -5
		}]
	}`,
}

var HIncrByFloat = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "mykey", "field", 10.50)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HIncrByFloat(ctx, "mykey", "field", 0.1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 10.6)

		r3, err := c.HIncrByFloat(ctx, "mykey", "field", -5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 5.6)

		r4, err := c.HSet(ctx, "mykey", "field", 5.0e3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(0))

		r5, err := c.HIncrByFloat(ctx, "mykey", "field", 2.0e2)
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
			"request": "HSET mykey field 10.5",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HINCRBYFLOAT mykey field 0.1",
			"response": "10.6"
		}, {
			"protocol": "redis",
			"request": "HINCRBYFLOAT mykey field -5",
			"response": "5.6"
		}, {
			"protocol": "redis",
			"request": "HSET mykey field 5000",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "HINCRBYFLOAT mykey field 200",
			"response": "5200"
		}]
	}`,
}

var HKeys = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HKeys(ctx, "myhash")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"field1", "field2"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash field1 Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HSET myhash field2 World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HKEYS myhash",
			"response": ["field1", "field2"]
		}]
	}`,
}

var HLen = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HLen(ctx, "myhash")
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
			"request": "HSET myhash field1 Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HSET myhash field2 World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HLEN myhash",
			"response": 2
		}]
	}`,
}

var HMGet = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HMGet(ctx, "myhash", "field1", "field2", "nofield")
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
			"request": "HSET myhash field1 Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HSET myhash field2 World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HMGET myhash field1 field2 nofield",
			"response": ["Hello", "World", null]
		}]
	}`,
}

var HSet = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HGet(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash field1 Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HGET myhash field1",
			"response": "Hello"
		}]
	}`,
}

var HSetNX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSetNX(ctx, "myhash", "field", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, 1)

		r2, err := c.HSetNX(ctx, "myhash", "field", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 0)

		r3, err := c.HGet(ctx, "myhash", "field")
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
			"request": "HSETNX myhash field Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HSETNX myhash field World",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "HGET myhash field",
			"response": "Hello"
		}]
	}`,
}

var HStrLen = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "f1", "HelloWorld", "f2", 99, "f3", -256)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(3))

		r2, err := c.HStrLen(ctx, "myhash", "f1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(10))

		r3, err := c.HStrLen(ctx, "myhash", "f2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(2))

		r4, err := c.HStrLen(ctx, "myhash", "f3")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(4))
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash f1 HelloWorld f2 99 f3 -256",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "HSTRLEN myhash f1",
			"response": 10
		}, {
			"protocol": "redis",
			"request": "HSTRLEN myhash f2",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "HSTRLEN myhash f3",
			"response": 4
		}]
	}`,
}

var HVals = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HVals(ctx, "myhash")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"Hello", "World"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "HSET myhash field1 Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HSET myhash field2 World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "HVALS myhash",
			"response": ["Hello", "World"]
		}]
	}`,
}
