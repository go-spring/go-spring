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

func LIndex(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func LInsert(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func LLen(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func LMove(t *testing.T, ctx context.Context, c redis.Client) {

	//r1, err := c.RPush(ctx, "mylist", "one")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r1, int64(1))
	//
	//r2, err := c.RPush(ctx, "mylist", "two")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r2, int64(2))
	//
	//r3, err := c.RPush(ctx, "mylist", "three")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r3, int64(3))
	//
	//r4, err := c.LMove(ctx, "mylist", "myotherlist", "RIGHT", "LEFT")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r4, "three")
	//
	//r5, err := c.LMove(ctx, "mylist", "myotherlist", "LEFT", "RIGHT")
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r5, "one")
	//
	//r6, err := c.LRange(ctx, "mylist", 0, -1)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r6, "two")
	//
	//r7, err := c.LRange(ctx, "myotherlist", 0, -1)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r7, []string{"three", "one"})
}

func LMPop(t *testing.T, ctx context.Context, c redis.Client) {

	//LMPOP
	//redis> LMPOP 2 non1 non2 LEFT COUNT 10
	//ERR Don't know what to do for "lmpop"
	//redis> LPUSH mylist "one" "two" "three" "four" "five"
	//(integer) 5
	//redis> LMPOP 1 mylist LEFT
	//ERR Don't know what to do for "lmpop"
	//redis> LRANGE mylist 0 -1
	//1) "five"
	//2) "four"
	//3) "three"
	//4) "two"
	//5) "one"
	//redis> LMPOP 1 mylist RIGHT COUNT 10
	//ERR Don't know what to do for "lmpop"
	//redis> LPUSH mylist "one" "two" "three" "four" "five"
	//(integer) 10
	//redis> LPUSH mylist2 "a" "b" "c" "d" "e"
	//(integer) 5
	//redis> LMPOP 2 mylist mylist2 right count 3
	//ERR Don't know what to do for "lmpop"
	//redis> LRANGE mylist 0 -1
	//1) "five"
	// 2) "four"
	// 3) "three"
	// 4) "two"
	// 5) "one"
	// 6) "five"
	// 7) "four"
	// 8) "three"
	// 9) "two"
	//10) "one"
	//redis> LMPOP 2 mylist mylist2 right count 5
	//ERR Don't know what to do for "lmpop"
	//redis> LMPOP 2 mylist mylist2 right count 10
	//ERR Don't know what to do for "lmpop"
	//redis> EXISTS mylist mylist2
	//(integer) 2
	//redis>
}

func LPop(t *testing.T, ctx context.Context, c redis.Client) {

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

	//r3, err := c.LPopN(ctx, "mylist", 2)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r3, []string{"two", "three"})
	//
	//r4, err := c.LRange(ctx, "mylist", 0, -1)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r4, []string{"four", "five"})
}

func LPos(t *testing.T, ctx context.Context, c redis.Client) {

	//r1, err := c.RPush(ctx, "mylist", 'a', 'b', 'c', 'd', 1, 2, 3, 4, 3, 3, 3)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r1, int64(11))
	//
	//// c.LPos(ctx, "mylist", "3")
	//
	//r3, err := c.LPos(ctx, "mylist", "3", 0, 2)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r3, []interface{}{int64(8), int64(9), int64(10)})

	//LPOS
	//redis> RPUSH mylist a b c d 1 2 3 4 3 3 3
	//(integer) 11
	//redis> LPOS mylist 3
	//(integer) 6
	//redis> LPOS mylist 3 COUNT 0 RANK 2
	//1) (integer) 8
	//2) (integer) 9
	//3) (integer) 10
	//redis>
}

func LPush(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func LPushX(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func LRange(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func LRem(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func LSet(t *testing.T, ctx context.Context, c redis.Client) {

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
	assert.Equal(t, r4, true)

	r5, err := c.LSet(ctx, "mylist", -2, "five")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, true)

	r6, err := c.LRange(ctx, "mylist", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r6, []string{"four", "five", "three"})
}

func LTrim(t *testing.T, ctx context.Context, c redis.Client) {

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
	assert.Equal(t, r4, true)

	r5, err := c.LRange(ctx, "mylist", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, r5, []string{"two", "three"})
}

func RPop(t *testing.T, ctx context.Context, c redis.Client) {

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

	//r3, err := c.RPopN(ctx, "mylist", 2)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r3, []string{"four", "three"})
	//
	//r4, err := c.LRange(ctx, "mylist", 0, -1)
	//if err != nil {
	//	t.Fatal(err)
	//}
	//assert.Equal(t, r4, []string{"one", "two"})
}

func RPopLPush(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func RPush(t *testing.T, ctx context.Context, c redis.Client) {

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
}

func RPushX(t *testing.T, ctx context.Context, c redis.Client) {

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
}
