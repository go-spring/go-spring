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
		assert.Equal(t, err, redis.ErrNil)
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "LPUSH mylist World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "LPUSH mylist Hello",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "LINDEX mylist 0",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "LINDEX mylist -1",
			"response": "World"
		}, {
			"protocol": "redis",
			"request": "LINDEX mylist 3",
			"response": "(nil)"
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist World",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "LINSERT mylist BEFORE World There",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["Hello", "There", "World"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "LPUSH mylist World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "LPUSH mylist Hello",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "LLEN mylist",
			"response": 2
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist two",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist three",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "LMOVE mylist myotherlist RIGHT LEFT",
			"response": "three"
		}, {
			"protocol": "redis",
			"request": "LMOVE mylist myotherlist LEFT RIGHT",
			"response": "one"
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["two"]
		}, {
			"protocol": "redis",
			"request": "LRANGE myotherlist 0 -1",
			"response": ["three", "one"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist one two three four five",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "LPOP mylist",
			"response": "one"
		}, {
			"protocol": "redis",
			"request": "LPOP mylist 2",
			"response": ["two", "three"]
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["four", "five"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist 97 98 99 100 1 2 3 4 3 3 3",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "LPOS mylist 3",
			"response": 6
		}, {
			"protocol": "redis",
			"request": "LPOS mylist 3 COUNT 0 RANK 2",
			"response": [8, 9, 10]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "LPUSH mylist world",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "LPUSH mylist hello",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["hello", "world"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "LPUSH mylist World",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "LPUSHX mylist Hello",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "LPUSHX myotherlist Hello",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["Hello", "World"]
		}, {
			"protocol": "redis",
			"request": "LRANGE myotherlist 0 -1",
			"response": []
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist two",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist three",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 0",
			"response": ["one"]
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist -3 2",
			"response": ["one", "two", "three"]
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist -100 100",
			"response": ["one", "two", "three"]
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 5 10",
			"response": []
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist hello",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist foo",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist hello",
			"response": 4
		}, {
			"protocol": "redis",
			"request": "LREM mylist -2 hello",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["hello", "foo"]
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
		assert.Equal(t, r4, redis.OK)

		r5, err := c.LSet(ctx, "mylist", -2, "five")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, redis.OK)

		r6, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r6, []string{"four", "five", "three"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist two",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist three",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "LSET mylist 0 four",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "LSET mylist -2 five",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["four", "five", "three"]
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
		assert.Equal(t, r4, redis.OK)

		r5, err := c.LRange(ctx, "mylist", 0, -1)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r5, []string{"two", "three"})
	},
	Data: `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist two",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist three",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "LTRIM mylist 1 -1",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["two", "three"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist one two three four five",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "RPOP mylist",
			"response": "five"
		}, {
			"protocol": "redis",
			"request": "RPOP mylist 2",
			"response": ["four", "three"]
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["one", "two"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist one",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist two",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist three",
			"response": 3
		}, {
			"protocol": "redis",
			"request": "RPOPLPUSH mylist myotherlist",
			"response": "three"
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["one", "two"]
		}, {
			"protocol": "redis",
			"request": "LRANGE myotherlist 0 -1",
			"response": ["three"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSH mylist world",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["hello", "world"]
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
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "RPUSH mylist Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "RPUSHX mylist World",
			"response": 2
		}, {
			"protocol": "redis",
			"request": "RPUSHX myotherlist World",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "LRANGE mylist 0 -1",
			"response": ["Hello", "World"]
		}, {
			"protocol": "redis",
			"request": "LRANGE myotherlist 0 -1",
			"response": []
		}]
	}`,
}
