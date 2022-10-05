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

package redis

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
)

func (c *Cases) HDel() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "foo")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HDel(ctx, "myhash", "field1")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.HDel(ctx, "myhash", "field2")
			assert.Nil(t, err)
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
}

func (c *Cases) HExists() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "foo")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HExists(ctx, "myhash", "field1")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.HExists(ctx, "myhash", "field2")
			assert.Nil(t, err)
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
				"Request": "HEXISTS myhash field1",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "HEXISTS myhash field2",
				"Response": "\"0\""
			}]
		}`,
	}
}

func (c *Cases) HGet() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "foo")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HGet(ctx, "myhash", "field1")
			assert.Nil(t, err)
			assert.Equal(t, r2, "foo")

			_, err = c.HGet(ctx, "myhash", "field2")
			assert.True(t, IsErrNil(err))
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
}

func (c *Cases) HGetAll() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HSet(ctx, "myhash", "field2", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.HGetAll(ctx, "myhash")
			assert.Nil(t, err)
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
}

func (c *Cases) HIncrBy() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field", 5)
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HIncrBy(ctx, "myhash", "field", 1)
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(6))

			r3, err := c.HIncrBy(ctx, "myhash", "field", -1)
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(5))

			r4, err := c.HIncrBy(ctx, "myhash", "field", -10)
			assert.Nil(t, err)
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
}

func (c *Cases) HIncrByFloat() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "mykey", "field", 10.50)
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HIncrByFloat(ctx, "mykey", "field", 0.1)
			assert.Nil(t, err)
			assert.Equal(t, r2, 10.6)

			r3, err := c.HIncrByFloat(ctx, "mykey", "field", -5)
			assert.Nil(t, err)
			assert.Equal(t, r3, 5.6)

			r4, err := c.HSet(ctx, "mykey", "field", 5.0e3)
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(0))

			r5, err := c.HIncrByFloat(ctx, "mykey", "field", 2.0e2)
			assert.Nil(t, err)
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
}

func (c *Cases) HKeys() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HSet(ctx, "myhash", "field2", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.HKeys(ctx, "myhash")
			assert.Nil(t, err)
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
}

func (c *Cases) HLen() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HSet(ctx, "myhash", "field2", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.HLen(ctx, "myhash")
			assert.Nil(t, err)
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
}

func (c *Cases) HMGet() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HSet(ctx, "myhash", "field2", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.HMGet(ctx, "myhash", "field1", "field2", "nofield")
			assert.Nil(t, err)
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
}

func (c *Cases) HSet() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HGet(ctx, "myhash", "field1")
			assert.Nil(t, err)
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
}

func (c *Cases) HSetNX() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSetNX(ctx, "myhash", "field", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HSetNX(ctx, "myhash", "field", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(0))

			r3, err := c.HGet(ctx, "myhash", "field")
			assert.Nil(t, err)
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
}

func (c *Cases) HStrLen() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "f1", "HelloWorld", "f2", 99, "f3", -256)
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(3))

			r2, err := c.HStrLen(ctx, "myhash", "f1")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(10))

			r3, err := c.HStrLen(ctx, "myhash", "f2")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(2))

			r4, err := c.HStrLen(ctx, "myhash", "f3")
			assert.Nil(t, err)
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
}

func (c *Cases) HVals() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.HSet(ctx, "myhash", "field2", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.HVals(ctx, "myhash")
			assert.Nil(t, err)
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
}
