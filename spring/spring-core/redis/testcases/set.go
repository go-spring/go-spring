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

//SADD
//redis> SADD myset "Hello"
//(integer) 1
//redis> SADD myset "World"
//(integer) 1
//redis> SADD myset "World"
//(integer) 0
//redis> SMEMBERS myset
//1) "World"
//2) "Hello"
//redis>
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

//SCARD
//redis> SADD myset "Hello"
//(integer) 1
//redis> SADD myset "World"
//(integer) 1
//redis> SCARD myset
//(integer) 2
//redis>
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

//SDIFF
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SDIFF key1 key2
//1) "b"
//2) "a"
//redis>
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

//SDIFFSTORE
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SDIFFSTORE key key1 key2
//(integer) 2
//redis> SMEMBERS key
//1) "b"
//2) "a"
//redis>
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

//SINTER
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SINTER key1 key2
//1) "c"
//redis>

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

//SINTERSTORE
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SINTERSTORE key key1 key2
//(integer) 1
//redis> SMEMBERS key
//1) "c"
//redis>

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

//SISMEMBER
//redis> SADD myset "one"
//(integer) 1
//redis> SISMEMBER myset "one"
//(integer) 1
//redis> SISMEMBER myset "two"
//(integer) 0
//redis>
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

//SMEMBERS
//redis> SADD myset "Hello"
//(integer) 1
//redis> SADD myset "World"
//(integer) 1
//redis> SMEMBERS myset
//1) "World"
//2) "Hello"
//redis>
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

//SMISMEMBER
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "one"
//(integer) 0
//redis> SMISMEMBER myset "one" "notamember"
//1) (integer) 1
//2) (integer) 0
//redis>
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

	// 可用版本>= 6.2.0.
	// r3, err := c.SMIsMember(ctx, "myset", "one", "notamember")
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// assert.Equal(t, r3, []bool{false, true})
}

//SMOVE
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "two"
//(integer) 1
//redis> SADD myotherset "three"
//(integer) 1
//redis> SMOVE myset myotherset "two"
//(integer) 1
//redis> SMEMBERS myset
//1) "one"
//redis> SMEMBERS myotherset
//1) "three"
//2) "two"
//redis>
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

//SPOP
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "two"
//(integer) 1
//redis> SADD myset "three"
//(integer) 1
//redis> SPOP myset
//"one"
//redis> SMEMBERS myset
//1) "three"
//2) "two"
//redis> SADD myset "four"
//(integer) 1
//redis> SADD myset "five"
//(integer) 1
//redis> SPOP myset 3
//1) "three"
//2) "four"
//3) "five"
//redis> SMEMBERS myset
//1) "two"
//redis>
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

	// SPOP 将随机元素从集合中移除并返回
	_, err = c.SPop(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}

	r5, err := c.SCard(ctx, "myset")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, int64(2))

	r6, err := c.SMembers(ctx, "myset")
	if err != nil {
		t.Fatal(r5)
	}
	assert.Equal(t, len(r6), 2)

	//sort.Strings(r6)
	//assert.Equal(t, r6, []string{"one", "two"})
}

//SRANDMEMBER
//redis> SADD myset one two three
//(integer) 3
//redis> SRANDMEMBER myset
//"two"
//redis> SRANDMEMBER myset 2
//1) "three"
//2) "two"
//redis> SRANDMEMBER myset -5
//1) "two"
//2) "one"
//3) "one"
//4) "three"
//5) "three"
//redis>
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

//SREM
//redis> SADD myset "one"
//(integer) 1
//redis> SADD myset "two"
//(integer) 1
//redis> SADD myset "three"
//(integer) 1
//redis> SREM myset "one"
//(integer) 1
//redis> SREM myset "four"
//(integer) 0
//redis> SMEMBERS myset
//1) "three"
//2) "two"
//redis>

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

//SUNION
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SUNION key1 key2
//1) "b"
//2) "c"
//3) "e"
//4) "a"
//5) "d"
//redis>
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

//SUNIONSTORE
//redis> SADD key1 "a"
//(integer) 1
//redis> SADD key1 "b"
//(integer) 1
//redis> SADD key1 "c"
//(integer) 1
//redis> SADD key2 "c"
//(integer) 1
//redis> SADD key2 "d"
//(integer) 1
//redis> SADD key2 "e"
//(integer) 1
//redis> SUNIONSTORE key key1 key2
//(integer) 5
//redis> SMEMBERS key
//1) "b"
//2) "c"
//3) "e"
//4) "a"
//5) "d"
//redis>
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
