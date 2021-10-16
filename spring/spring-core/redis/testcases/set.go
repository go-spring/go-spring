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

func SAdd(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "myset", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "myset", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(0))

	r4, err := c.SMembers(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r4)
	assert.Equal(t, r4, []string{"Hello", "World"})
}

func SCard(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "myset", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SCard(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(2))
}

func SDiff(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "key1", "a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "key1", "b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "key1", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SAdd(ctx, "key2", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.SAdd(ctx, "key2", "d")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.SAdd(ctx, "key2", "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, int64(1))

	r7, err := c.SDiff(ctx, "key1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r7)
	assert.Equal(t, r7, []string{"a", "b"})
}

func SDiffStore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "key1", "a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "key1", "b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "key1", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SAdd(ctx, "key2", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.SAdd(ctx, "key2", "d")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.SAdd(ctx, "key2", "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, int64(1))

	r7, err := c.SDiffStore(ctx, "key", "key1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r7, int64(2))

	r8, err := c.SMembers(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r8)
	assert.Equal(t, r8, []string{"a", "b"})
}

func SInter(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "key1", "a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "key1", "b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "key1", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SAdd(ctx, "key2", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.SAdd(ctx, "key2", "d")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.SAdd(ctx, "key2", "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, int64(1))

	r7, err := c.SInter(ctx, "key1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r7, []string{"c"})
}

func SInterStore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "key1", "a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "key1", "b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "key1", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SAdd(ctx, "key2", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.SAdd(ctx, "key2", "d")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.SAdd(ctx, "key2", "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, int64(1))

	r7, err := c.SInterStore(ctx, "key", "key1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r7, int64(1))

	r8, err := c.SMembers(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r8, []string{"c"})
}

func SIsMember(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SIsMember(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SIsMember(ctx, "myset", "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(0))
}

func SMembers(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "myset", "World")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SMembers(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r3)
	assert.Equal(t, r3, []string{"Hello", "World"})
}

func SMIsMember(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(0))

	r3, err := c.SMIsMember(ctx, "myset", "one", "notamember")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, []bool{true, false})
}

func SMove(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "myset", "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "myotherset", "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SMove(ctx, "myset", "myotherset", "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, r4)

	r5, err := c.SMembers(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, []string{"one"})

	r6, err := c.SMembers(ctx, "myotherset")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r6)
	assert.Equal(t, r6, []string{"three", "two"})
}

func SPop(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "myset", "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "myset", "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SPop(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}

	r5, err := c.SMembers(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}

	r6 := append([]string{r4}, r5...)
	sort.Strings(r6)
	assert.Equal(t, r6, []string{"one", "three", "two"})
}

func SRandMember(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "one", "two", "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(3))

	_, err = c.SRandMember(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}

	r3, err := c.SRandMemberN(ctx, "myset", 2)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r3), 2)

	r4, err := c.SRandMemberN(ctx, "myset", -5)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, len(r4), 5)
}

func SRem(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "myset", "two")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "myset", "three")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SRem(ctx, "myset", "one")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.SRem(ctx, "myset", "four")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(0))

	r6, err := c.SMembers(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r6)
	assert.Equal(t, r6, []string{"three", "two"})
}

func SUnion(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "key1", "a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "key1", "b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "key1", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SAdd(ctx, "key2", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.SAdd(ctx, "key2", "d")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.SAdd(ctx, "key2", "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, int64(1))

	r7, err := c.SUnion(ctx, "key1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r7)
	assert.Equal(t, r7, []string{"a", "b", "c", "d", "e"})
}

func SUnionStore(t *testing.T, ctx context.Context, c redis.Client) {

	r1, err := c.SAdd(ctx, "key1", "a")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r1, int64(1))

	r2, err := c.SAdd(ctx, "key1", "b")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r2, int64(1))

	r3, err := c.SAdd(ctx, "key1", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r3, int64(1))

	r4, err := c.SAdd(ctx, "key2", "c")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r4, int64(1))

	r5, err := c.SAdd(ctx, "key2", "d")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(1))

	r6, err := c.SAdd(ctx, "key2", "e")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, int64(1))

	r7, err := c.SUnionStore(ctx, "key", "key1", "key2")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r7, int64(5))

	r8, err := c.SMembers(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(r8)
	assert.Equal(t, r8, []string{"a", "b", "c", "d", "e"})
}
