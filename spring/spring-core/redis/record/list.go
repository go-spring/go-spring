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

package record

import (
	"testing"

	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/redis/testcases"
	"github.com/go-spring/spring-core/redis/testdata"
)

func LIndex(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LIndex, testdata.LIndex)
}

func LInsert(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LInsert, testdata.LInsert)
}

func LLen(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LLen, testdata.LLen)
}

func LMove(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LMove, testdata.LMove)
}

func LPop(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LPop, testdata.LPop)
}

func LPos(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LPos, testdata.LPos)
}

func LPush(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LPush, testdata.LPush)
}

func LPushX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LPushX, testdata.LPushX)
}

func LRange(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LRange, testdata.LRange)
}

func LRem(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LRem, testdata.LRem)
}

func LSet(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LSet, testdata.LSet)
}

func LTrim(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.LTrim, testdata.LTrim)
}

func RPop(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.RPop, testdata.RPop)
}

func RPopLPush(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.RPopLPush, testdata.RPopLPush)
}

func RPush(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.RPush, testdata.RPush)
}

func RPushX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.RPushX, testdata.RPushX)
}
