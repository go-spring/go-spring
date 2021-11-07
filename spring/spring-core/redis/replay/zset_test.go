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
	"testing"

	"github.com/go-spring/spring-core/redis/testcases"
	"github.com/go-spring/spring-core/redis/testdata"
)

func TestZAdd(t *testing.T) {
	RunCase(t, testcases.ZAdd, testdata.ZAdd)
}

func TestZCard(t *testing.T) {
	RunCase(t, testcases.ZCard, testdata.ZCard)
}

func TestZCount(t *testing.T) {
	RunCase(t, testcases.ZCount, testdata.ZCount)
}

func TestZDiff(t *testing.T) {
	RunCase(t, testcases.ZDiff, testdata.ZDiff)
}

func TestZIncrBy(t *testing.T) {
	RunCase(t, testcases.ZIncrBy, testdata.ZIncrBy)
}

func TestZInter(t *testing.T) {
	RunCase(t, testcases.ZInter, testdata.ZInter)
}

func TestZLexCount(t *testing.T) {
	RunCase(t, testcases.ZLexCount, testdata.ZLexCount)
}

func TestZMScore(t *testing.T) {
	RunCase(t, testcases.ZMScore, testdata.ZMScore)
}

func TestZPopMax(t *testing.T) {
	RunCase(t, testcases.ZPopMax, testdata.ZPopMax)
}

func TestZPopMin(t *testing.T) {
	RunCase(t, testcases.ZPopMin, testdata.ZPopMin)
}

func TestZRandMember(t *testing.T) {
	RunCase(t, testcases.ZRandMember, testdata.ZRandMember)
}

func TestZRange(t *testing.T) {
	RunCase(t, testcases.ZRange, testdata.ZRange)
}

func TestZRangeByLex(t *testing.T) {
	RunCase(t, testcases.ZRangeByLex, testdata.ZRangeByLex)
}

func TestZRangeByScore(t *testing.T) {
	RunCase(t, testcases.ZRangeByScore, testdata.ZRangeByScore)
}

func TestZRank(t *testing.T) {
	RunCase(t, testcases.ZRank, testdata.ZRank)
}

func TestZRem(t *testing.T) {
	RunCase(t, testcases.ZRem, testdata.ZRem)
}

func TestZRemRangeByLex(t *testing.T) {
	RunCase(t, testcases.ZRemRangeByLex, testdata.ZRemRangeByLex)
}

func TestZRemRangeByRank(t *testing.T) {
	RunCase(t, testcases.ZRemRangeByRank, testdata.ZRemRangeByRank)
}

func TestZRemRangeByScore(t *testing.T) {
	RunCase(t, testcases.ZRemRangeByScore, testdata.ZRemRangeByScore)
}

func TestZRevRange(t *testing.T) {
	RunCase(t, testcases.ZRevRange, testdata.ZRevRange)
}

func TestZRevRangeByLex(t *testing.T) {
	RunCase(t, testcases.ZRevRangeByLex, testdata.ZRevRangeByLex)
}

func TestZRevRangeByScore(t *testing.T) {
	RunCase(t, testcases.ZRevRangeByScore, testdata.ZRevRangeByScore)
}

func TestZRevRank(t *testing.T) {
	RunCase(t, testcases.ZRevRank, testdata.ZRevRank)
}

func TestZScore(t *testing.T) {
	RunCase(t, testcases.ZScore, testdata.ZScore)
}

func TestZUnion(t *testing.T) {
	RunCase(t, testcases.ZUnion, testdata.ZUnion)
}

func TestZUnionStore(t *testing.T) {
	RunCase(t, testcases.ZUnionStore, testdata.ZUnionStore)
}
