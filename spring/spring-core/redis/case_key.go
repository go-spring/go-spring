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
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
)

func (c *Cases) Del() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "key1", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Set(ctx, "key2", "World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r2))

			r3, err := c.Del(ctx, "key1", "key2", "key3")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(2))
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
				"Request": "DEL key1 key2 key3",
				"Response": "\"2\""
			}]
		}`,
	}
}

func (c *Cases) Dump() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", 10)
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Dump(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, "\u0000\xC0\n\t\u0000\xBEm\u0006\x89Z(\u0000\n")
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
				"Request": "DUMP mykey",
				"Response": "\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\""
			}]
		}`,
	}
}

func (c *Cases) Exists() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "key1", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Exists(ctx, "key1")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.Exists(ctx, "nosuchkey")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(0))

			r4, err := c.Set(ctx, "key2", "World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r4))

			r5, err := c.Exists(ctx, "key1", "key2", "nosuchkey")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(2))
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
				"Request": "EXISTS key1",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "EXISTS nosuchkey",
				"Response": "\"0\""
			}, {
				"Protocol": "REDIS",
				"Request": "SET key2 World",
				"Response": "\"OK\""
			}, {
				"Protocol": "REDIS",
				"Request": "EXISTS key1 key2 nosuchkey",
				"Response": "\"2\""
			}]
		}`,
	}
}

func (c *Cases) Expire() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Expire(ctx, "mykey", 10)
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(10))

			r4, err := c.Set(ctx, "mykey", "Hello World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r4))

			r5, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(-1))
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
				"Request": "EXPIRE mykey 10",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "TTL mykey",
				"Response": "\"10\""
			}, {
				"Protocol": "REDIS",
				"Request": "SET mykey \"Hello World\"",
				"Response": "\"OK\""
			}, {
				"Protocol": "REDIS",
				"Request": "TTL mykey",
				"Response": "\"-1\""
			}]
		}`,
	}
}

func (c *Cases) ExpireAt() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Exists(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.ExpireAt(ctx, "mykey", 1293840000)
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.Exists(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(0))
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
				"Request": "EXISTS mykey",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "EXPIREAT mykey 1293840000",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "EXISTS mykey",
				"Response": "\"0\""
			}]
		}`,
	}
}

func (c *Cases) Keys() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.MSet(ctx, "firstname", "Jack", "lastname", "Stuntman", "age", 35)
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Keys(ctx, "*name*")
			assert.Nil(t, err)
			sort.Strings(r2)
			assert.Equal(t, r2, []string{"firstname", "lastname"})

			r3, err := c.Keys(ctx, "a??")
			assert.Nil(t, err)
			assert.Equal(t, r3, []string{"age"})

			r4, err := c.Keys(ctx, "*")
			assert.Nil(t, err)
			sort.Strings(r4)
			assert.Equal(t, r4, []string{"age", "firstname", "lastname"})
		},
		Skip: true,
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "MSET firstname Jack lastname Stuntman age 35",
				"Response": "\"OK\""
			}, {
				"Protocol": "REDIS",
				"Request": "KEYS *name*",
				"Response": "\"lastname\",\"firstname\""
			}, {
				"Protocol": "REDIS",
				"Request": "KEYS a??",
				"Response": "\"age\""
			}, {
				"Protocol": "REDIS",
				"Request": "KEYS *",
				"Response": "\"age\",\"lastname\",\"firstname\""
			}]
		}`,
	}
}

func (c *Cases) Persist() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Expire(ctx, "mykey", 10)
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(10))

			r4, err := c.Persist(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(1))

			r5, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r5, int64(-1))
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
				"Request": "EXPIRE mykey 10",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "TTL mykey",
				"Response": "\"10\""
			}, {
				"Protocol": "REDIS",
				"Request": "PERSIST mykey",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "TTL mykey",
				"Response": "\"-1\""
			}]
		}`,
	}
}

func (c *Cases) PExpire() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.PExpire(ctx, "mykey", 1500)
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.True(t, r3 >= 1 && r3 <= 2)

			r4, err := c.PTTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.True(t, r4 >= 1400 && r4 <= 1500)
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
				"Request": "PEXPIRE mykey 1500",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "TTL mykey",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "PTTL mykey",
				"Response": "\"1499\""
			}]
		}`,
	}
}

func (c *Cases) PExpireAt() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.PExpireAt(ctx, "mykey", 1555555555005)
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(-2))

			r4, err := c.PTTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(-2))
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
				"Request": "PEXPIREAT mykey 1555555555005",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "TTL mykey",
				"Response": "\"-2\""
			}, {
				"Protocol": "REDIS",
				"Request": "PTTL mykey",
				"Response": "\"-2\""
			}]
		}`,
	}
}

func (c *Cases) PTTL() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Expire(ctx, "mykey", 1)
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.PTTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.True(t, r3 >= 900 && r3 <= 1000)
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
				"Request": "EXPIRE mykey 1",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "PTTL mykey",
				"Response": "\"1000\""
			}]
		}`,
	}
}

func (c *Cases) RandomKey() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey1", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Set(ctx, "mykey2", "world")
			assert.Nil(t, err)
			assert.True(t, IsOK(r2))

			r3, err := c.RandomKey(ctx)
			assert.Nil(t, err)
			assert.InSlice(t, r3, []string{"mykey1", "mykey2"})
		},
		Skip: true,
		Data: "",
	}
}

func (c *Cases) Rename() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Rename(ctx, "mykey", "myotherkey")
			assert.Nil(t, err)
			assert.True(t, IsOK(r2))

			r3, err := c.Get(ctx, "myotherkey")
			assert.Nil(t, err)
			assert.Equal(t, r3, "Hello")
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
				"Request": "RENAME mykey myotherkey",
				"Response": "\"OK\""
			}, {
				"Protocol": "REDIS",
				"Request": "GET myotherkey",
				"Response": "\"Hello\""
			}]
		}`,
	}
}

func (c *Cases) RenameNX() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Set(ctx, "myotherkey", "World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r2))

			r3, err := c.RenameNX(ctx, "mykey", "myotherkey")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(0))

			r4, err := c.Get(ctx, "myotherkey")
			assert.Nil(t, err)
			assert.Equal(t, r4, "World")
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
				"Request": "SET myotherkey World",
				"Response": "\"OK\""
			}, {
				"Protocol": "REDIS",
				"Request": "RENAMENX mykey myotherkey",
				"Response": "\"0\""
			}, {
				"Protocol": "REDIS",
				"Request": "GET myotherkey",
				"Response": "\"World\""
			}]
		}`,
	}
}

func (c *Cases) Touch() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "key1", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Set(ctx, "key2", "World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r2))

			r3, err := c.Touch(ctx, "key1", "key2")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(2))
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
				"Request": "TOUCH key1 key2",
				"Response": "\"2\""
			}]
		}`,
	}
}

func (c *Cases) TTL() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Expire(ctx, "mykey", 10)
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(10))
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
				"Request": "EXPIRE mykey 10",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "TTL mykey",
				"Response": "\"10\""
			}]
		}`,
	}
}

func (c *Cases) Type() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "key1", "value")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.LPush(ctx, "key2", "value")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(1))

			r3, err := c.SAdd(ctx, "key3", "value")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(1))

			r4, err := c.Type(ctx, "key1")
			assert.Nil(t, err)
			assert.Equal(t, r4, "string")

			r5, err := c.Type(ctx, "key2")
			assert.Nil(t, err)
			assert.Equal(t, r5, "list")

			r6, err := c.Type(ctx, "key3")
			assert.Nil(t, err)
			assert.Equal(t, r6, "set")
		},
		Data: `
		{
			"Session": "df3b64266ebe4e63a464e135000a07cd",
			"Actions": [{
				"Protocol": "REDIS",
				"Request": "SET key1 value",
				"Response": "\"OK\""
			}, {
				"Protocol": "REDIS",
				"Request": "LPUSH key2 value",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "SADD key3 value",
				"Response": "\"1\""
			}, {
				"Protocol": "REDIS",
				"Request": "TYPE key1",
				"Response": "\"string\""
			}, {
				"Protocol": "REDIS",
				"Request": "TYPE key2",
				"Response": "\"list\""
			}, {
				"Protocol": "REDIS",
				"Request": "TYPE key3",
				"Response": "\"set\""
			}]
		}`,
	}
}
