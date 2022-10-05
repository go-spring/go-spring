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

	"github.com/go-spring/spring-core/redis/test/cases"
)

func TestZAdd(t *testing.T) {
	RunCase(t, cases.ZAdd)
}

func TestZCard(t *testing.T) {
	RunCase(t, cases.ZCard)
}

func TestZCount(t *testing.T) {
	RunCase(t, cases.ZCount)
}

func TestZDiff(t *testing.T) {
	RunCase(t, cases.ZDiff)
}

func TestZIncrBy(t *testing.T) {
	RunCase(t, cases.ZIncrBy)
}

func TestZInter(t *testing.T) {
	RunCase(t, cases.ZInter)
}

func TestZLexCount(t *testing.T) {
	RunCase(t, cases.ZLexCount)
}

func TestZMScore(t *testing.T) {
	RunCase(t, cases.ZMScore)
}

func TestZPopMax(t *testing.T) {
	RunCase(t, cases.ZPopMax)
}

func TestZPopMin(t *testing.T) {
	RunCase(t, cases.ZPopMin)
}

func TestZRandMember(t *testing.T) {
	RunCase(t, cases.ZRandMember)
}

func TestZRange(t *testing.T) {
	RunCase(t, cases.ZRange)
}

func TestZRangeByLex(t *testing.T) {
	RunCase(t, cases.ZRangeByLex)
}

func TestZRangeByScore(t *testing.T) {
	RunCase(t, cases.ZRangeByScore)
}

func TestZRank(t *testing.T) {
	RunCase(t, cases.ZRank)
}

func TestZRem(t *testing.T) {
	RunCase(t, cases.ZRem)
}

func TestZRemRangeByLex(t *testing.T) {
	RunCase(t, cases.ZRemRangeByLex)
}

func TestZRemRangeByRank(t *testing.T) {
	RunCase(t, cases.ZRemRangeByRank)
}

func TestZRemRangeByScore(t *testing.T) {
	RunCase(t, cases.ZRemRangeByScore)
}

func TestZRevRange(t *testing.T) {
	RunCase(t, cases.ZRevRange)
}

func TestZRevRangeByLex(t *testing.T) {
	RunCase(t, cases.ZRevRangeByLex)
}

func TestZRevRangeByScore(t *testing.T) {
	RunCase(t, cases.ZRevRangeByScore)
}

func TestZRevRank(t *testing.T) {
	RunCase(t, cases.ZRevRank)
}

func TestZScore(t *testing.T) {
	RunCase(t, cases.ZScore)
}

func TestZUnion(t *testing.T) {
	RunCase(t, cases.ZUnion)
}

func TestZUnionStore(t *testing.T) {
	RunCase(t, cases.ZUnionStore)
}
