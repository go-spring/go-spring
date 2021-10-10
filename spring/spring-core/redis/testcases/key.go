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
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

func Del(t *testing.T, ctx context.Context, c redis.Client) {

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

	r3, err := c.Del(ctx, "key1", "key2", "key3")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(2))
}

func Dump(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", 10)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Dump(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, "\u0000\xC0\n\t\u0000\xBEm\u0006\x89Z(\u0000\n")
}

func Exists(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "key1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Exists(ctx, "key1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.Exists(ctx, "nosuchkey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(0))

	r4, err := c.Set(ctx, "key2", "World")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, r4, true)

	r5, err := c.Exists(ctx, "key1", "key2", "nosuchkey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(2))
}

func Expire(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Expire(ctx, "mykey", 10)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.TTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(10))

	r4, err := c.Set(ctx, "mykey", "Hello World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, true)

	r5, err := c.TTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(-1))
}

func ExpireAt(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Exists(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.ExpireAt(ctx, "mykey", 1293840000)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, true)

	r4, err := c.Exists(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(0))
}

func Keys(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.MSet(ctx, "firstname", "Jack", "lastname", "Stuntman", "age", 35)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Keys(ctx, "*name*")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r2)
	assert.Equal(t, r2, []string{"firstname", "lastname"})

	r3, err := c.Keys(ctx, "a??")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, []string{"age"})

	r4, err := c.Keys(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r4)
	assert.Equal(t, r4, []string{"age", "firstname", "lastname"})
}

func Persist(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Expire(ctx, "mykey", 10)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.TTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(10))

	r4, err := c.Persist(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, true)

	r5, err := c.TTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(-1))
}

func PExpire(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.PExpire(ctx, "mykey", 1500)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.TTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, r3 >= 1 && r3 <= 2)

	r4, err := c.PTTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, r4 >= 1400 && r4 <= 1500)
}

func PExpireAt(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.PExpireAt(ctx, "mykey", 1555555555005)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.TTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(-2))

	r4, err := c.PTTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(-2))
}

func PTTL(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Expire(ctx, "mykey", 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.PTTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, r3 >= 990 && r3 <= 1000)
}

func Rename(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Rename(ctx, "mykey", "myotherkey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.Get(ctx, "myotherkey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, "Hello")
}

func RenameNX(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Set(ctx, "myotherkey", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.RenameNX(ctx, "mykey", "myotherkey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, false)

	r4, err := c.Get(ctx, "myotherkey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, "World")
}

func Restore(t *testing.T, ctx context.Context, c redis.Client) {

	//RESTORE
	//redis> DEL mykey
	//0
	//redis> RESTORE mykey 0 "\n\x17\x17\x00\x00\x00\x12\x00\x00\x00\x03\x00\
	//                        x00\xc0\x01\x00\x04\xc0\x02\x00\x04\xc0\x03\x00\
	//                        xff\x04\x00u#<\xc0;.\xe9\xdd"
	//OK
	//redis> TYPE mykey
	//list
	//redis> LRANGE mykey 0 -1
	//1) "1"
	//2) "2"
	//3) "3"
}

func Touch(t *testing.T, ctx context.Context, c redis.Client) {

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

	r3, err := c.Touch(ctx, "key1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(2))
}

func TTL(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.Expire(ctx, "mykey", 10)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.TTL(ctx, "mykey")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(10))
}

func Type(t *testing.T, ctx context.Context, c redis.Client) {

	//TYPE
	//redis> SET key1 "value"
	//"OK"
	//redis> LPUSH key2 "value"
	//(integer) 1
	//redis> SADD key3 "value"
	//(integer) 1
	//redis> TYPE key1
	//"string"
	//redis> TYPE key2
	//"list"
	//redis> TYPE key3
	//"set"
	//redis>
}
