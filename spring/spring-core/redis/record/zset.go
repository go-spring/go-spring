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

func ZAdd(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZAdd, testdata.ZAdd)
}

func ZCard(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZCard, testdata.ZCard)
}

func ZCount(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZCount, testdata.ZCount)
}

func ZDiff(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZDiff, testdata.ZDiff)
}

func ZIncrBy(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZIncrBy, testdata.ZIncrBy)
}

func ZInter(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZInter, testdata.ZInter)
}

func ZLexCount(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZLexCount, testdata.ZLexCount)
}

func ZMScore(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZMScore, testdata.ZMScore)
}

func ZPopMax(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZPopMax, testdata.ZPopMax)
}

func ZPopMin(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZPopMin, testdata.ZPopMin)
}

func ZRandMember(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRandMember, "skip")
}

func ZRange(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRange, testdata.ZRange)
}

func ZRangeByLex(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRangeByLex, testdata.ZRangeByLex)
}

func ZRangeByScore(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRangeByScore, testdata.ZRangeByScore)
}

func ZRank(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRank, testdata.ZRank)
}

func ZRem(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRem, testdata.ZRem)
}

func ZRemRangeByLex(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRemRangeByLex, testdata.ZRemRangeByLex)
}

func ZRemRangeByRank(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRemRangeByRank, testdata.ZRemRangeByRank)
}

func ZRemRangeByScore(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRemRangeByScore, testdata.ZRemRangeByScore)
}

func ZRevRange(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRevRange, testdata.ZRevRange)
}

func ZRevRangeByLex(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRevRangeByLex, testdata.ZRevRangeByLex)
}

func ZRevRangeByScore(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRevRangeByScore, testdata.ZRevRangeByScore)
}

func ZRevRank(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZRevRank, testdata.ZRevRank)
}

func ZScore(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZScore, testdata.ZScore)
}

func ZUnion(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZUnion, testdata.ZUnion)
}

func ZUnionStore(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ZUnionStore, testdata.ZUnionStore)
}
