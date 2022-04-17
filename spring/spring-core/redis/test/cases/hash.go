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
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "foo")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HDel(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HashCommand().HDel(ctx, "myhash", "field2")
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
			"Request": "HSET myhash field1 foo",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HDEL myhash field1",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HDEL myhash field2",
			"Response": "\"0\""
		}]
	}`,
}

var HExists = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "foo")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HExists(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 1)

		r3, err := c.HashCommand().HExists(ctx, "myhash", "field2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 0)
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash field1 foo",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HEXISTS myhash field1",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HEXISTS myhash field2",
			"Response": "\"0\""
		}]
	}`,
}

var HGet = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "foo")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HGet(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "foo")

		_, err = c.HashCommand().HGet(ctx, "myhash", "field2")
		assert.True(t, redis.IsErrNil(err))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash field1 foo",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HGET myhash field1",
			"Response": "\"foo\""
		}, {
			"Protocol": "REDIS",
			"Request": "HGET myhash field2",
			"Response": "NULL"
		}]
	}`,
}

var HGetAll = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HashCommand().HGetAll(ctx, "myhash")
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
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash field1 Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSET myhash field2 World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HGETALL myhash",
			"Response": "\"field1\",\"Hello\",\"field2\",\"World\""
		}]
	}`,
}

var HIncrBy = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field", 5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HIncrBy(ctx, "myhash", "field", 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(6))

		r3, err := c.HashCommand().HIncrBy(ctx, "myhash", "field", -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(5))

		r4, err := c.HashCommand().HIncrBy(ctx, "myhash", "field", -10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(-5))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash field 5",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HINCRBY myhash field 1",
			"Response": "\"6\""
		}, {
			"Protocol": "REDIS",
			"Request": "HINCRBY myhash field -1",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "HINCRBY myhash field -10",
			"Response": "\"-5\""
		}]
	}`,
}

var HIncrByFloat = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "mykey", "field", 10.50)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HIncrByFloat(ctx, "mykey", "field", 0.1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 10.6)

		r3, err := c.HashCommand().HIncrByFloat(ctx, "mykey", "field", -5)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, 5.6)

		r4, err := c.HashCommand().HSet(ctx, "mykey", "field", 5.0e3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(0))

		r5, err := c.HashCommand().HIncrByFloat(ctx, "mykey", "field", 2.0e2)
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
			"Request": "HSET mykey field 10.5",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HINCRBYFLOAT mykey field 0.1",
			"Response": "\"10.6\""
		}, {
			"Protocol": "REDIS",
			"Request": "HINCRBYFLOAT mykey field -5",
			"Response": "\"5.6\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSET mykey field 5000",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "HINCRBYFLOAT mykey field 200",
			"Response": "\"5200\""
		}]
	}`,
}

var HKeys = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HashCommand().HKeys(ctx, "myhash")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"field1", "field2"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash field1 Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSET myhash field2 World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HKEYS myhash",
			"Response": "\"field1\",\"field2\""
		}]
	}`,
}

var HLen = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HashCommand().HLen(ctx, "myhash")
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
			"Request": "HSET myhash field1 Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSET myhash field2 World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HLEN myhash",
			"Response": "\"2\""
		}]
	}`,
}

var HMGet = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HashCommand().HMGet(ctx, "myhash", "field1", "field2", "nofield")
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
			"Request": "HSET myhash field1 Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSET myhash field2 World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HMGET myhash field1 field2 nofield",
			"Response": "\"Hello\",\"World\",NULL"
		}]
	}`,
}

var HSet = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HGet(ctx, "myhash", "field1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "Hello")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash field1 Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HGET myhash field1",
			"Response": "\"Hello\""
		}]
	}`,
}

var HSetNX = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSetNX(ctx, "myhash", "field", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, 1)

		r2, err := c.HashCommand().HSetNX(ctx, "myhash", "field", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, 0)

		r3, err := c.HashCommand().HGet(ctx, "myhash", "field")
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
			"Request": "HSETNX myhash field Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSETNX myhash field World",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "HGET myhash field",
			"Response": "\"Hello\""
		}]
	}`,
}

var HStrLen = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "f1", "HelloWorld", "f2", 99, "f3", -256)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(3))

		r2, err := c.HashCommand().HStrLen(ctx, "myhash", "f1")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(10))

		r3, err := c.HashCommand().HStrLen(ctx, "myhash", "f2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(2))

		r4, err := c.HashCommand().HStrLen(ctx, "myhash", "f3")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(4))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash f1 HelloWorld f2 99 f3 -256",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSTRLEN myhash f1",
			"Response": "\"10\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSTRLEN myhash f2",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSTRLEN myhash f3",
			"Response": "\"4\""
		}]
	}`,
}

var HVals = Case{
	Func: func(t *testing.T, ctx context.Context, c *redis.Client) {

		r1, err := c.HashCommand().HSet(ctx, "myhash", "field1", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.HashCommand().HSet(ctx, "myhash", "field2", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.HashCommand().HVals(ctx, "myhash")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"Hello", "World"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "HSET myhash field1 Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HSET myhash field2 World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "HVALS myhash",
			"Response": "\"Hello\",\"World\""
		}]
	}`,
}
