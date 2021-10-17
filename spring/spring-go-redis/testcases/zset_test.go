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
	"testing"

	"github.com/go-spring/spring-core/redis/testcases"
)

func TestZAdd(t *testing.T) {
	RunCase(t, testcases.ZAdd)
}

func TestZCard(t *testing.T) {
	RunCase(t, testcases.ZCard)
}

func TestZCount(t *testing.T) {
	RunCase(t, testcases.ZCount)
}

func TestZDiff(t *testing.T) {
	RunCase(t, testcases.ZDiff)
}

func TestZIncrBy(t *testing.T) {
	RunCase(t, testcases.ZIncrBy)
}

func TestZInter(t *testing.T) {
	RunCase(t, testcases.ZInter)
}

func TestZLexCount(t *testing.T) {
	RunCase(t, testcases.ZLexCount)
}

func TestZMScore(t *testing.T) {
	RunCase(t, testcases.ZMScore)
}

func TestZPopMax(t *testing.T) {
	RunCase(t, testcases.ZPopMax)
}

func TestZPopMin(t *testing.T) {
	RunCase(t, testcases.ZPopMin)
}

func TestZRandMember(t *testing.T) {
	RunCase(t, testcases.ZRandMember)
}

func TestZRange(t *testing.T) {
	RunCase(t, testcases.ZRange)
}

func TestZRangeByLex(t *testing.T) {
	RunCase(t, testcases.ZRangeByLex)
}

func TestZRangeByScore(t *testing.T) {
	RunCase(t, testcases.ZRangeByScore)
}

func TestZRank(t *testing.T) {
	RunCase(t, testcases.ZRank)
}

func TestZRem(t *testing.T) {
	RunCase(t, testcases.ZRem)
}

func TestZRemRangeByLex(t *testing.T) {
	RunCase(t, testcases.ZRemRangeByLex)
}

func TestZRemRangeByRank(t *testing.T) {
	RunCase(t, testcases.ZRemRangeByRank)
}

func TestZRemRangeByScore(t *testing.T) {
	RunCase(t, testcases.ZRemRangeByScore)
}

func TestZRevRange(t *testing.T) {
	RunCase(t, testcases.ZRevRange)
}

func TestZRevRangeByLex(t *testing.T) {
	RunCase(t, testcases.ZRevRangeByLex)
}

func TestZRevRangeByScore(t *testing.T) {
	RunCase(t, testcases.ZRevRangeByScore)
}

func TestZRevRank(t *testing.T) {
	RunCase(t, testcases.ZRevRank)
}

func TestZScore(t *testing.T) {
	RunCase(t, testcases.ZScore)
}

func TestZUnion(t *testing.T) {
	RunCase(t, testcases.ZUnion)
}

func TestZUnionStore(t *testing.T) {
	RunCase(t, testcases.ZUnionStore)
}
