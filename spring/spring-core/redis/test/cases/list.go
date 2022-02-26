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

var LIndex = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.LPush(ctx, "mylist", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.LPush(ctx, "mylist", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.LIndex(ctx, "mylist", 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, "Hello")

		r4, err := c.LIndex(ctx, "mylist", -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, "World")

		_, err = c.LIndex(ctx, "mylist", 3)
		assert.True(t, redis.IsErrNil(err))
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "LPUSH mylist World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPUSH mylist Hello",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "LINDEX mylist 0",
			"Response": "\"Hello\""
		}, {
			"Protocol": "REDIS",
			"Request": "LINDEX mylist -1",
			"Response": "\"World\""
		}, {
			"Protocol": "REDIS",
			"Request": "LINDEX mylist 3",
			"Response": "NULL"
		}]
	}`,
}

var LInsert = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.LInsertBefore(ctx, "mylist", "World", "There")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(3))

		r4, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"Hello", "There", "World"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist World",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "LINSERT mylist BEFORE World There",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"Hello\",\"There\",\"World\""
		}]
	}`,
}

var LLen = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.LPush(ctx, "mylist", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.LPush(ctx, "mylist", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.LLen(ctx, "mylist")
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
			"Request": "LPUSH mylist World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPUSH mylist Hello",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "LLEN mylist",
			"Response": "\"2\""
		}]
	}`,
}

var LMove = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.RPush(ctx, "mylist", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(3))

		r4, err := c.LMove(ctx, "mylist", "myotherlist", "RIGHT", "LEFT")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, "three")

		r5, err := c.LMove(ctx, "mylist", "myotherlist", "LEFT", "RIGHT")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, "one")

		r6, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"two"})

		r7, err := c.LRange(ctx, "myotherlist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []string{"three", "one"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist two",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist three",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "LMOVE mylist myotherlist RIGHT LEFT",
			"Response": "\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "LMOVE mylist myotherlist LEFT RIGHT",
			"Response": "\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE myotherlist 0 -1",
			"Response": "\"three\",\"one\""
		}]
	}`,
}

var LPop = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "one", "two", "three", "four", "five")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(5))

		r2, err := c.LPop(ctx, "mylist")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "one")

		r3, err := c.LPopN(ctx, "mylist", 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"two", "three"})

		r4, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"four", "five"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist one two three four five",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPOP mylist",
			"Response": "\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPOP mylist 2",
			"Response": "\"two\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"four\",\"five\""
		}]
	}`,
}

var LPos = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", 'a', 'b', 'c', 'd', 1, 2, 3, 4, 3, 3, 3)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(11))

		r2, err := c.LPos(ctx, "mylist", 3)
		if err != nil {
			return
		}
		assert.Equal(t, r2, int64(6))

		r3, err := c.LPosN(ctx, "mylist", "3", 0, "RANK", 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []int64{8, 9, 10})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist 97 98 99 100 1 2 3 4 3 3 3",
			"Response": "\"11\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPOS mylist 3",
			"Response": "\"6\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPOS mylist 3 COUNT 0 RANK 2",
			"Response": "\"8\",\"9\",\"10\""
		}]
	}`,
}

var LPush = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.LPush(ctx, "mylist", "world")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.LPush(ctx, "mylist", "hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"hello", "world"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "LPUSH mylist world",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPUSH mylist hello",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"hello\",\"world\""
		}]
	}`,
}

var LPushX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.LPush(ctx, "mylist", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.LPushX(ctx, "mylist", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.LPushX(ctx, "myotherlist", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))

		r4, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"Hello", "World"})

		r5, err := c.LRange(ctx, "myotherlist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "LPUSH mylist World",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPUSHX mylist Hello",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "LPUSHX myotherlist Hello",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"Hello\",\"World\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE myotherlist 0 -1",
			"Response": ""
		}]
	}`,
}

var LRange = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.RPush(ctx, "mylist", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(3))

		r4, err := c.LRange(ctx, "mylist", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"one"})

		r5, err := c.LRange(ctx, "mylist", -3, 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one", "two", "three"})

		r6, err := c.LRange(ctx, "mylist", -100, 100)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"one", "two", "three"})

		r7, err := c.LRange(ctx, "mylist", 5, 10)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r7, []string{})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist two",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist three",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 0",
			"Response": "\"one\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist -3 2",
			"Response": "\"one\",\"two\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist -100 100",
			"Response": "\"one\",\"two\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 5 10",
			"Response": ""
		}]
	}`,
}

var LRem = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.RPush(ctx, "mylist", "foo")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(3))

		r4, err := c.RPush(ctx, "mylist", "hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, int64(4))

		r5, err := c.LRem(ctx, "mylist", -2, "hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, int64(2))

		r6, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"hello", "foo"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist hello",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist foo",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist hello",
			"Response": "\"4\""
		}, {
			"Protocol": "REDIS",
			"Request": "LREM mylist -2 hello",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"hello\",\"foo\""
		}]
	}`,
}

var LSet = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.RPush(ctx, "mylist", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(3))

		r4, err := c.LSet(ctx, "mylist", 0, "four")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r4))

		r5, err := c.LSet(ctx, "mylist", -2, "five")
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r5))

		r6, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"four", "five", "three"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist two",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist three",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "LSET mylist 0 four",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "LSET mylist -2 five",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"four\",\"five\",\"three\""
		}]
	}`,
}

var LTrim = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.RPush(ctx, "mylist", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(3))

		r4, err := c.LTrim(ctx, "mylist", 1, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.True(t, redis.IsOK(r4))

		r5, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"two", "three"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist two",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist three",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "LTRIM mylist 1 -1",
			"Response": "\"OK\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"two\",\"three\""
		}]
	}`,
}

var RPop = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "one", "two", "three", "four", "five")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(5))

		r2, err := c.RPop(ctx, "mylist")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, "five")

		r3, err := c.RPopN(ctx, "mylist", 2)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"four", "three"})

		r4, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"one", "two"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist one two three four five",
			"Response": "\"5\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPOP mylist",
			"Response": "\"five\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPOP mylist 2",
			"Response": "\"four\",\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"one\",\"two\""
		}]
	}`,
}

var RPopLPush = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "one")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "two")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.RPush(ctx, "mylist", "three")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(3))

		r4, err := c.RPopLPush(ctx, "mylist", "myotherlist")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, "three")

		r5, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"one", "two"})

		r6, err := c.LRange(ctx, "myotherlist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"three"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist one",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist two",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist three",
			"Response": "\"3\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPOPLPUSH mylist myotherlist",
			"Response": "\"three\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"one\",\"two\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE myotherlist 0 -1",
			"Response": "\"three\""
		}]
	}`,
}

var RPush = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPush(ctx, "mylist", "world")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, []string{"hello", "world"})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSH mylist world",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"hello\",\"world\""
		}]
	}`,
}

var RPushX = Case{
	Func: func(t *testing.T, ctx context.Context, c redis.Client) {

		r1, err := c.RPush(ctx, "mylist", "Hello")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r1, int64(1))

		r2, err := c.RPushX(ctx, "mylist", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r2, int64(2))

		r3, err := c.RPushX(ctx, "myotherlist", "World")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r3, int64(0))

		r4, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r4, []string{"Hello", "World"})

		r5, err := c.LRange(ctx, "myotherlist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{})
	},
	Data: `
	{
		"Session": "df3b64266ebe4e63a464e135000a07cd",
		"Actions": [{
			"Protocol": "REDIS",
			"Request": "RPUSH mylist Hello",
			"Response": "\"1\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSHX mylist World",
			"Response": "\"2\""
		}, {
			"Protocol": "REDIS",
			"Request": "RPUSHX myotherlist World",
			"Response": "\"0\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE mylist 0 -1",
			"Response": "\"Hello\",\"World\""
		}, {
			"Protocol": "REDIS",
			"Request": "LRANGE myotherlist 0 -1",
			"Response": ""
		}]
	}`,
}
