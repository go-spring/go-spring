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
	"github.com/go-spring/spring-core/redis/test/cases"
)

func ZAdd(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZAdd)
}

func ZCard(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZCard)
}

func ZCount(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZCount)
}

func ZDiff(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZDiff)
}

func ZIncrBy(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZIncrBy)
}

func ZInter(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZInter)
}

func ZLexCount(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZLexCount)
}

func ZMScore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZMScore)
}

func ZPopMax(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZPopMax)
}

func ZPopMin(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZPopMin)
}

func ZRandMember(t *testing.T, d redis.Driver) {
	// RunCase(t, d, cases.ZRandMember)
}

func ZRange(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRange)
}

func ZRangeByLex(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRangeByLex)
}

func ZRangeByScore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRangeByScore)
}

func ZRank(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRank)
}

func ZRem(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRem)
}

func ZRemRangeByLex(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRemRangeByLex)
}

func ZRemRangeByRank(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRemRangeByRank)
}

func ZRemRangeByScore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRemRangeByScore)
}

func ZRevRange(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRevRange)
}

func ZRevRangeByLex(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRevRangeByLex)
}

func ZRevRangeByScore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRevRangeByScore)
}

func ZRevRank(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZRevRank)
}

func ZScore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZScore)
}

func ZUnion(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZUnion)
}

func ZUnionStore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ZUnionStore)
}
