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

	"github.com/go-spring/spring-core/redis/test/record"
)

func TestZAdd(t *testing.T) {
	RunCase(t, record.ZAdd)
}

func TestZCard(t *testing.T) {
	RunCase(t, record.ZCard)
}

func TestZCount(t *testing.T) {
	RunCase(t, record.ZCount)
}

func TestZDiff(t *testing.T) {
	RunCase(t, record.ZDiff)
}

func TestZIncrBy(t *testing.T) {
	RunCase(t, record.ZIncrBy)
}

func TestZInter(t *testing.T) {
	RunCase(t, record.ZInter)
}

func TestZLexCount(t *testing.T) {
	RunCase(t, record.ZLexCount)
}

func TestZMScore(t *testing.T) {
	RunCase(t, record.ZMScore)
}

func TestZPopMax(t *testing.T) {
	RunCase(t, record.ZPopMax)
}

func TestZPopMin(t *testing.T) {
	RunCase(t, record.ZPopMin)
}

func TestZRandMember(t *testing.T) {
	RunCase(t, record.ZRandMember)
}

func TestZRange(t *testing.T) {
	RunCase(t, record.ZRange)
}

func TestZRangeByLex(t *testing.T) {
	RunCase(t, record.ZRangeByLex)
}

func TestZRangeByScore(t *testing.T) {
	RunCase(t, record.ZRangeByScore)
}

func TestZRank(t *testing.T) {
	RunCase(t, record.ZRank)
}

func TestZRem(t *testing.T) {
	RunCase(t, record.ZRem)
}

func TestZRemRangeByLex(t *testing.T) {
	RunCase(t, record.ZRemRangeByLex)
}

func TestZRemRangeByRank(t *testing.T) {
	RunCase(t, record.ZRemRangeByRank)
}

func TestZRemRangeByScore(t *testing.T) {
	RunCase(t, record.ZRemRangeByScore)
}

func TestZRevRange(t *testing.T) {
	RunCase(t, record.ZRevRange)
}

func TestZRevRangeByLex(t *testing.T) {
	RunCase(t, record.ZRevRangeByLex)
}

func TestZRevRangeByScore(t *testing.T) {
	RunCase(t, record.ZRevRangeByScore)
}

func TestZRevRank(t *testing.T) {
	RunCase(t, record.ZRevRank)
}

func TestZScore(t *testing.T) {
	RunCase(t, record.ZScore)
}

func TestZUnion(t *testing.T) {
	RunCase(t, record.ZUnion)
}

func TestZUnionStore(t *testing.T) {
	RunCase(t, record.ZUnionStore)
}
