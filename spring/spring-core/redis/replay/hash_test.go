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

package replay

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/redis"
)

func HDel(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "foo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HDel(ctx, "myhash", "field1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.HDel(ctx, "myhash", "field2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(0))
}

func HExists(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "foo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HExists(ctx, "myhash", "field1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, true)

	r3, err := c.HExists(ctx, "myhash", "field2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, false)
}

func HGet(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "foo")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HGet(ctx, "myhash", "field1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, "foo")

	_, err = c.HGet(ctx, "myhash", "field2")
	assert.Equal(t, err, redis.ErrNil)
}

func HGetAll(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HSet(ctx, "myhash", "field2", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.HGetAll(ctx, "myhash")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, map[string]string{
		"field1": "Hello",
		"field2": "World",
	})
}

func HIncrBy(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field", 5)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HIncrBy(ctx, "myhash", "field", 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(6))

	r3, err := c.HIncrBy(ctx, "myhash", "field", -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(5))

	r4, err := c.HIncrBy(ctx, "myhash", "field", -10)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(-5))
}

func HIncrByFloat(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "mykey", "field", 10.50)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HIncrByFloat(ctx, "mykey", "field", 0.1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, 10.6)

	r3, err := c.HIncrByFloat(ctx, "mykey", "field", -5)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, 5.6)

	r4, err := c.HSet(ctx, "mykey", "field", 5.0e3)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(0))

	r5, err := c.HIncrByFloat(ctx, "mykey", "field", 2.0e2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, float64(5200))
}

func HKeys(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HSet(ctx, "myhash", "field2", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.HKeys(ctx, "myhash")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, []string{"field1", "field2"})
}

func HLen(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HSet(ctx, "myhash", "field2", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.HLen(ctx, "myhash")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(2))
}

func HMGet(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HSet(ctx, "myhash", "field2", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.HMGet(ctx, "myhash", "field1", "field2", "nofield")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, []interface{}{"Hello", "World", nil})
}

func HSet(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HGet(ctx, "myhash", "field1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, "Hello")
}

func HSetNX(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSetNX(ctx, "myhash", "field", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, true)

	r2, err := c.HSetNX(ctx, "myhash", "field", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, false)

	r3, err := c.HGet(ctx, "myhash", "field")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, "Hello")
}

func HStrLen(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "f1", "HelloWorld", "f2", 99, "f3", -256)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(3))

	r2, err := c.HStrLen(ctx, "myhash", "f1")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(10))

	r3, err := c.HStrLen(ctx, "myhash", "f2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(2))

	r4, err := c.HStrLen(ctx, "myhash", "f3")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(4))
}

func HVals(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.HSet(ctx, "myhash", "field1", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.HSet(ctx, "myhash", "field2", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.HVals(ctx, "myhash")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, []string{"Hello", "World"})
}
