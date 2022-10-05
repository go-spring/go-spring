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

func (c *Cases) Append() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Exists(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(0))

			r2, err := c.Append(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(5))

			r3, err := c.Append(ctx, "mykey", " World")
			assert.Nil(t, err)
			assert.Equal(t, r3, int64(11))

			r4, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
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
}

func (c *Cases) Decr() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "10")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Decr(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(9))

			r3, err := c.Set(ctx, "mykey", "234293482390480948029348230948")
			assert.Nil(t, err)
			assert.True(t, IsOK(r3))

			_, err = c.Decr(ctx, "mykey")
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
}

func (c *Cases) DecrBy() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "10")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.DecrBy(ctx, "mykey", 3)
			assert.Nil(t, err)
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
}

func (c *Cases) Get() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			_, err := c.Get(ctx, "nonexisting")
			assert.True(t, IsErrNil(err))

			r2, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r2))

			r3, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
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
}

func (c *Cases) GetDel() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.GetDel(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, "Hello")

			_, err = c.Get(ctx, "mykey")
			assert.True(t, IsErrNil(err))
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
}

func (c *Cases) GetRange() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "This is a string")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.GetRange(ctx, "mykey", 0, 3)
			assert.Nil(t, err)
			assert.Equal(t, r2, "This")

			r3, err := c.GetRange(ctx, "mykey", -3, -1)
			assert.Nil(t, err)
			assert.Equal(t, r3, "ing")

			r4, err := c.GetRange(ctx, "mykey", 0, -1)
			assert.Nil(t, err)
			assert.Equal(t, r4, "This is a string")

			r5, err := c.GetRange(ctx, "mykey", 10, 100)
			assert.Nil(t, err)
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
}

func (c *Cases) GetSet() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Incr(ctx, "mycounter")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.GetSet(ctx, "mycounter", "0")
			assert.Nil(t, err)
			assert.Equal(t, r2, "1")

			r3, err := c.Get(ctx, "mycounter")
			assert.Nil(t, err)
			assert.Equal(t, r3, "0")

			r4, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r4))

			r5, err := c.GetSet(ctx, "mykey", "World")
			assert.Nil(t, err)
			assert.Equal(t, r5, "Hello")

			r6, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
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
}

func (c *Cases) Incr() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "10")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Incr(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(11))

			r3, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
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
}

func (c *Cases) IncrBy() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "10")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.IncrBy(ctx, "mykey", 5)
			assert.Nil(t, err)
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
}

func (c *Cases) IncrByFloat() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", 10.50)
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.IncrByFloat(ctx, "mykey", 0.1)
			assert.Nil(t, err)
			assert.Equal(t, r2, 10.6)

			r3, err := c.IncrByFloat(ctx, "mykey", -5)
			assert.Nil(t, err)
			assert.Equal(t, r3, 5.6)

			r4, err := c.Set(ctx, "mykey", 5.0e3)
			assert.Nil(t, err)
			assert.True(t, IsOK(r4))

			r5, err := c.IncrByFloat(ctx, "mykey", 2.0e2)
			assert.Nil(t, err)
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
}

func (c *Cases) MGet() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "key1", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Set(ctx, "key2", "World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r2))

			r3, err := c.MGet(ctx, "key1", "key2", "nonexisting")
			assert.Nil(t, err)
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
}

func (c *Cases) MSet() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.MSet(ctx, "key1", "Hello", "key2", "World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Get(ctx, "key1")
			assert.Nil(t, err)
			assert.Equal(t, r2, "Hello")

			r3, err := c.Get(ctx, "key2")
			assert.Nil(t, err)
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
}

func (c *Cases) MSetNX() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.MSetNX(ctx, "key1", "Hello", "key2", "there")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.MSetNX(ctx, "key2", "new", "key3", "world")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(0))

			r3, err := c.MGet(ctx, "key1", "key2", "key3")
			assert.Nil(t, err)
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
}

func (c *Cases) PSetEX() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.PSetEX(ctx, "mykey", "Hello", 1000)
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.PTTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.True(t, r2 <= 1000 && r2 >= 900)

			r3, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
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
}

func (c *Cases) Set() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, "Hello")

			r3, err := c.SetEX(ctx, "anotherkey", "will expire in a minute", 60)
			assert.Nil(t, err)
			assert.True(t, IsOK(r3))
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
}

func (c *Cases) SetEX() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SetEX(ctx, "mykey", "Hello", 10)
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.TTL(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(10))

			r3, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
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
}

func (c *Cases) SetNX() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.SetNX(ctx, "mykey", "Hello")
			assert.Nil(t, err)
			assert.Equal(t, r1, int64(1))

			r2, err := c.SetNX(ctx, "mykey", "World")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(0))

			r3, err := c.Get(ctx, "mykey")
			assert.Nil(t, err)
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
}

func (c *Cases) SetRange() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "key1", "Hello World")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.SetRange(ctx, "key1", 6, "Redis")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(11))

			r3, err := c.Get(ctx, "key1")
			assert.Nil(t, err)
			assert.Equal(t, r3, "Hello Redis")

			r4, err := c.SetRange(ctx, "key2", 6, "Redis")
			assert.Nil(t, err)
			assert.Equal(t, r4, int64(11))

			r5, err := c.Get(ctx, "key2")
			assert.Nil(t, err)
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
}

func (c *Cases) StrLen() *Case {
	return &Case{
		Func: func(t *testing.T, ctx context.Context, c *Client) {

			r1, err := c.Set(ctx, "mykey", "Hello world")
			assert.Nil(t, err)
			assert.True(t, IsOK(r1))

			r2, err := c.StrLen(ctx, "mykey")
			assert.Nil(t, err)
			assert.Equal(t, r2, int64(11))

			r3, err := c.StrLen(ctx, "nonexisting")
			assert.Nil(t, err)
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
}
