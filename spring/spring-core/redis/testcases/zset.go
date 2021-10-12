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

	"github.com/go-spring/spring-core/redis"
)

// ZADD
// redis> ZADD myzset 1 "one"
// (integer) 1
// redis> ZADD myzset 1 "uno"
// (integer) 1
// redis> ZADD myzset 2 "two" 3 "three"
// (integer) 2
// redis> ZRANGE myzset 0 -1 WITHSCORES
// 1) "one"
// 2) "1"
// 3) "uno"
// 4) "1"
// 5) "two"
// 6) "2"
// 7) "three"
// 8) "3"
// redis>
func ZAdd(t *testing.T, ctx context.Context, c redis.Client) {

	// r1, err := c.ZAdd(ctx, "myzset", &redis.ZItem{
	// 	Score:  1,
	// 	Member: "one",
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// assert.Equal(t, r1, int64(1))

	// r2, err := c.ZAdd(ctx, "myzset", &redis.ZItem{
	// 	Score:  1,
	// 	Member: "uno",
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// assert.Equal(t, r2, int64(1))

	// r3, err := c.ZAdd(ctx, "myzset", &redis.ZItem{
	// 	Score:  2,
	// 	Member: "two",
	// }, &redis.ZItem{
	// 	Score:  3,
	// 	Member: "three",
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// assert.Equal(t, r3, int64(2))
}
