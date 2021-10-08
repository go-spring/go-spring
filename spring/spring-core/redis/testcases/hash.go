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

//HDEL
//redis> HSET myhash field1 "foo"
//(integer) 1
//redis> HDEL myhash field1
//(integer) 1
//redis> HDEL myhash field2
//(integer) 0
//redis>

//HEXISTS
//redis> HSET myhash field1 "foo"
//(integer) 1
//redis> HEXISTS myhash field1
//(integer) 1
//redis> HEXISTS myhash field2
//(integer) 0
//redis>

//HGET
//redis> HSET myhash field1 "foo"
//(integer) 1
//redis> HGET myhash field1
//"foo"
//redis> HGET myhash field2
//(nil)
//redis>

//HGETALL
//redis> HSET myhash field1 "Hello"
//(integer) 1
//redis> HSET myhash field2 "World"
//(integer) 1
//redis> HGETALL myhash
//1) "field1"
//2) "Hello"
//3) "field2"
//4) "World"
//redis>

//HINCRBY
//redis> HSET myhash field 5
//(integer) 1
//redis> HINCRBY myhash field 1
//(integer) 6
//redis> HINCRBY myhash field -1
//(integer) 5
//redis> HINCRBY myhash field -10
//(integer) -5
//redis>

//HINCRBYFLOAT
//redis> HSET mykey field 10.50
//(integer) 1
//redis> HINCRBYFLOAT mykey field 0.1
//"10.6"
//redis> HINCRBYFLOAT mykey field -5
//"5.6"
//redis> HSET mykey field 5.0e3
//(integer) 0
//redis> HINCRBYFLOAT mykey field 2.0e2
//"5200"
//redis>

//HKEYS
//redis> HSET myhash field1 "Hello"
//(integer) 1
//redis> HSET myhash field2 "World"
//(integer) 1
//redis> HKEYS myhash
//1) "field1"
//2) "field2"
//redis>

//HLEN
//redis> HSET myhash field1 "Hello"
//(integer) 1
//redis> HSET myhash field2 "World"
//(integer) 1
//redis> HLEN myhash
//(integer) 2
//redis>

//HMGET
//redis> HSET myhash field1 "Hello"
//(integer) 1
//redis> HSET myhash field2 "World"
//(integer) 1
//redis> HMGET myhash field1 field2 nofield
//1) "Hello"
//2) "World"
//3) (nil)
//redis>

func HMSet(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HMSet(ctx, "myhash", "field1", "Hello", "field2", "World")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r1, true)

	r2, err := c.HGet(ctx, "myhash", "field1")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r2, "Hello")

	r3, err := c.HGet(ctx, "myhash", "field2")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r3, "World")
}

func HSet(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HGet(ctx, "myhash", "field1")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r2, "Hello")
}

func HSetNX(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSetNX(ctx, "myhash", "field", "Hello")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r1, true)

	r2, err := c.HSetNX(ctx, "myhash", "field", "World")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r2, false)

	r3, err := c.HGet(ctx, "myhash", "field")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r3, "Hello")
}

func HStrLen(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HMSet(ctx, "myhash", "f1", "HelloWorld", "f2", 99, "f3", -256)
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r1, true)

	r2, err := c.HStrLen(ctx, "myhash", "f1")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r2, int64(10))

	r3, err := c.HStrLen(ctx, "myhash", "f2")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r3, int64(2))

	r4, err := c.HStrLen(ctx, "myhash", "f3")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r4, int64(4))
}

func HVals(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HSet(ctx, "myhash", "field2", "World")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.HVals(ctx, "myhash")
	if err != nil {
		t.Fatal()
	}
	assert.Equal(t, r3, []string{"Hello", "World"})
}
