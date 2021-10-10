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

package testcases

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

func Append(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func Decr(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "10")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Decr(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(9))

	r3, err := c.Set(ctx, "mykey", "234293482390480948029348230948")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, true)

	_, err = c.Decr(ctx, "mykey")
	assert.Error(t, err, "ERR value is not an integer or out of range")
}

func DecrBy(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "10")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.DecrBy(ctx, "mykey", 3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(7))
}

func Get(t *testing.T, ctx context.Context, c redis.Client) {

	_, err := c.Get(ctx, "nonexisting")
	assert.Equal(t, err, redis.ErrNil)

	r2, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.Get(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, "Hello")
}

func GetDel(t *testing.T, ctx context.Context, c redis.Client) {

	// TODO 需要使用 6.2.0 以上的 redis 服务器。

	//r1, err := c.Set(ctx, "mykey", "Hello")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r1, true)
	//
	//r2, err := c.GetDel(ctx, "mykey")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r2, "Hello")
	//
	//_, err = c.Get(ctx, "mykey")
	//assert.Equal(t, err, redis.ErrNil)
}

func GetRange(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "This is a string")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
}

func GetSet(t *testing.T, ctx context.Context, c redis.Client) {

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
	assert.Equal(t, r4, true)

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
}

func Incr(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "10")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
}

func IncrBy(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "10")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.IncrBy(ctx, "mykey", 5)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(15))
}

func IncrByFloat(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", 10.50)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
	assert.Equal(t, r4, true)

	r5, err := c.IncrByFloat(ctx, "mykey", 2.0e2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, float64(5200))
}

func MGet(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "key1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Set(ctx, "key2", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.MGet(ctx, "key1", "key2", "nonexisting")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, []interface{}{"Hello", "World", nil})
}

func MSet(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.MSet(ctx, "key1", "Hello", "key2", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
}

func MSetNX(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.MSetNX(ctx, "key1", "Hello", "key2", "there")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.MSetNX(ctx, "key2", "new", "key3", "world")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, false)

	r3, err := c.MGet(ctx, "key1", "key2", "key3")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, []interface{}{"Hello", "there", nil})
}

func PSetEX(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.PSetEX(ctx, "mykey", "Hello", 1000)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
}

func Set(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Get(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, "Hello")

	r3, err := c.SetEX(ctx, "anotherkey", "will expire in a minute", 60)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, true)
}

func SetEX(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SetEX(ctx, "mykey", "Hello", 10)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
}

func SetNX(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SetNX(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.SetNX(ctx, "mykey", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, false)

	r3, err := c.Get(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, "Hello")
}

func SetRange(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "key1", "Hello World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
}

func StrLen(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello world")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

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
}
