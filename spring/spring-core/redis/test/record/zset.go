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

func ZAdd(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZAdd)
}

func ZCard(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZCard)
}

func ZCount(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZCount)
}

func ZDiff(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZDiff)
}

func ZIncrBy(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZIncrBy)
}

func ZInter(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZInter)
}

func ZLexCount(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZLexCount)
}

func ZMScore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZMScore)
}

func ZPopMax(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZPopMax)
}

func ZPopMin(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZPopMin)
}

func ZRandMember(t *testing.T, conn redis.ConnPool) {
	// RunCase(t, conn, cases.ZRandMember)
}

func ZRange(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRange)
}

func ZRangeByLex(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRangeByLex)
}

func ZRangeByScore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRangeByScore)
}

func ZRank(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRank)
}

func ZRem(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRem)
}

func ZRemRangeByLex(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRemRangeByLex)
}

func ZRemRangeByRank(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRemRangeByRank)
}

func ZRemRangeByScore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRemRangeByScore)
}

func ZRevRange(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRevRange)
}

func ZRevRangeByLex(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRevRangeByLex)
}

func ZRevRangeByScore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRevRangeByScore)
}

func ZRevRank(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZRevRank)
}

func ZScore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZScore)
}

func ZUnion(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZUnion)
}

func ZUnionStore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ZUnionStore)
}
