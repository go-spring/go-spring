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

func ZAdd(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZAdd)
}

func ZCard(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZCard)
}

func ZCount(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZCount)
}

func ZDiff(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZDiff)
}

func ZIncrBy(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZIncrBy)
}

func ZInter(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZInter)
}

func ZLexCount(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZLexCount)
}

func ZMScore(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZMScore)
}

func ZPopMax(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZPopMax)
}

func ZPopMin(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZPopMin)
}

func ZRandMember(t *testing.T, c redis.Client) {
	// RunCase(t, c, cases.ZRandMember)
}

func ZRange(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRange)
}

func ZRangeByLex(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRangeByLex)
}

func ZRangeByScore(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRangeByScore)
}

func ZRank(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRank)
}

func ZRem(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRem)
}

func ZRemRangeByLex(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRemRangeByLex)
}

func ZRemRangeByRank(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRemRangeByRank)
}

func ZRemRangeByScore(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRemRangeByScore)
}

func ZRevRange(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRevRange)
}

func ZRevRangeByLex(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRevRangeByLex)
}

func ZRevRangeByScore(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRevRangeByScore)
}

func ZRevRank(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZRevRank)
}

func ZScore(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZScore)
}

func ZUnion(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZUnion)
}

func ZUnionStore(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.ZUnionStore)
}
