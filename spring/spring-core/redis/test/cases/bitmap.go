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

var BitCount = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "foobar")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.BitCount(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(26))

		r3, err := c.BitCount(ctx, "mykey", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(4))

		r4, err := c.BitCount(ctx, "mykey", 1, 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(6))
	},
	Data: `{
	  "Session": "df3b64266ebe4e63a464e135000a07cd",
	  "Actions": [
		{
		  "Protocol": "REDIS",
		  "Request": "SET mykey foobar",
		  "Response": "\"OK\""
		},
		{
		  "Protocol": "REDIS",
		  "Request": "BITCOUNT mykey",
		  "Response": "\"26\""
		},
		{
		  "Protocol": "REDIS",
		  "Request": "BITCOUNT mykey 0 0",
		  "Response": "\"4\""
		},
		{
		  "Protocol": "REDIS",
		  "Request": "BITCOUNT mykey 1 1",
		  "Response": "\"6\""
		}
	  ]
	}`,
}

var BitOpAnd = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "key1", "foobar")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.Set(ctx, "key2", "abcdef")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r2))

		r3, err := c.BitOpAnd(ctx, "dest", "key1", "key2")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(6))

		r4, err := c.Get(ctx, "dest")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, "`bc`ab")
	},
	Data: `{
	  "Session": "df3b64266ebe4e63a464e135000a07cd",
	  "Actions": [
		{
		  "Protocol": "REDIS",
		  "Request": "SET key1 foobar",
		  "Response": "\"OK\""
		},
		{
		  "Protocol": "REDIS",
		  "Request": "SET key2 abcdef",
		  "Response": "\"OK\""
		},
		{
		  "Protocol": "REDIS",
		  "Request": "BITOP AND dest key1 key2",
		  "Response": "\"6\""
		},
		{
		  "Protocol": "REDIS",
		  "Request": "GET dest",
		  "Response": "\"` + "`bc`ab" + `\""
		}
	  ]
	}`,
}

var BitPos = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.Set(ctx, "mykey", "\xff\xf0\x00")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r1))

		r2, err := c.BitPos(ctx, "mykey", 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(12))

		r3, err := c.Set(ctx, "mykey", "\x00\xff\xf0")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r3))

		r4, err := c.BitPos(ctx, "mykey", 1, 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(8))

		r5, err := c.BitPos(ctx, "mykey", 1, 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(16))

		r6, err := c.Set(ctx, "mykey", "\x00\x00\x00")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r6))

		r7, err := c.BitPos(ctx, "mykey", 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, int64(-1))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SET mykey \"\\xff\\xf0\\x00\"",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "BITPOS mykey 0",
			"Response": "\"12\""
		}, {
			"Protocol": "REDIS",
			"Request": "SET mykey \"\\x00\\xff\\xf0\"",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "BITPOS mykey 1 0",
			"Response": "\"8\""
		}, {
			"Protocol": "REDIS",
			"Request": "BITPOS mykey 1 2",
			"Response": "\"16\""
		}, {
			"Protocol": "REDIS",
			"Request": "SET mykey \u0000\u0000\u0000",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "BITPOS mykey 1",
			"Response": "\"-1\""
		}]
	}`,
}

var GetBit = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SetBit(ctx, "mykey", 7, 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(0))

		r2, err := c.GetBit(ctx, "mykey", 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(0))

		r3, err := c.GetBit(ctx, "mykey", 7)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(1))

		r4, err := c.GetBit(ctx, "mykey", 100)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(0))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SETBIT mykey 7 1",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETBIT mykey 0",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETBIT mykey 7",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "GETBIT mykey 100",
			"Response": "\"0\""
		}]
	}`,
}

var SetBit = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.SetBit(ctx, "mykey", 7, 1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(0))

		r2, err := c.SetBit(ctx, "mykey", 7, 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(1))

		r3, err := c.Get(ctx, "mykey")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "\u0000")
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "SETBIT mykey 7 1",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "SETBIT mykey 7 0",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "GET mykey",
			"Response": "\"\\x00\""
		}]
	}`,
}
