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

func BitCount(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "foobar")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, redis.OK)

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
}

func BitOpAnd(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "key1", "foobar")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, redis.OK)

	r2, err := c.Set(ctx, "key2", "abcdef")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, redis.OK)

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
}

func BitPos(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.Set(ctx, "mykey", "\xff\xf0\x00")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, redis.OK)

	r2, err := c.BitPos(ctx, "mykey", 0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(12))

	r3, err := c.Set(ctx, "mykey", "\x00\xff\xf0")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, redis.OK)

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
	assert.Equal(t, r6, redis.OK)

	r7, err := c.BitPos(ctx, "mykey", 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r7, int64(-1))
}

func GetBit(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func SetBit(t *testing.T, ctx context.Context, c redis.Client) {

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
}
