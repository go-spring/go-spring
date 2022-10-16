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

package redis_test

import (
	"testing"

	"github.com/go-spring/spring-core/redis"
)

func TestZAdd(t *testing.T) {
	runCase(t, new(redis.Cases).ZAdd())
}

func TestZCard(t *testing.T) {
	runCase(t, new(redis.Cases).ZCard())
}

func TestZCount(t *testing.T) {
	runCase(t, new(redis.Cases).ZCount())
}

func TestZDiff(t *testing.T) {
	runCase(t, new(redis.Cases).ZDiff())
}

func TestZIncrBy(t *testing.T) {
	runCase(t, new(redis.Cases).ZIncrBy())
}

func TestZInter(t *testing.T) {
	runCase(t, new(redis.Cases).ZInter())
}

func TestZLexCount(t *testing.T) {
	runCase(t, new(redis.Cases).ZLexCount())
}

func TestZMScore(t *testing.T) {
	runCase(t, new(redis.Cases).ZMScore())
}

func TestZPopMax(t *testing.T) {
	runCase(t, new(redis.Cases).ZPopMax())
}

func TestZPopMaxN(t *testing.T) {
	runCase(t, new(redis.Cases).ZPopMaxN())
}

func TestZPopMin(t *testing.T) {
	runCase(t, new(redis.Cases).ZPopMin())
}

func TestZPopMinN(t *testing.T) {
	runCase(t, new(redis.Cases).ZPopMinN())
}

func TestZRandMember(t *testing.T) {
	runCase(t, new(redis.Cases).ZRandMember())
}

func TestZRandMemberN(t *testing.T) {
	runCase(t, new(redis.Cases).ZRandMemberN())
}

func TestZRange(t *testing.T) {
	runCase(t, new(redis.Cases).ZRange())
}

func TestZRangeByLex(t *testing.T) {
	runCase(t, new(redis.Cases).ZRangeByLex())
}

func TestZRangeByScore(t *testing.T) {
	runCase(t, new(redis.Cases).ZRangeByScore())
}

func TestZRank(t *testing.T) {
	runCase(t, new(redis.Cases).ZRank())
}

func TestZRem(t *testing.T) {
	runCase(t, new(redis.Cases).ZRem())
}

func TestZRemRangeByLex(t *testing.T) {
	runCase(t, new(redis.Cases).ZRemRangeByLex())
}

func TestZRemRangeByRank(t *testing.T) {
	runCase(t, new(redis.Cases).ZRemRangeByRank())
}

func TestZRemRangeByScore(t *testing.T) {
	runCase(t, new(redis.Cases).ZRemRangeByScore())
}

func TestZRevRange(t *testing.T) {
	runCase(t, new(redis.Cases).ZRevRange())
}

func TestZRevRangeWithScores(t *testing.T) {
	runCase(t, new(redis.Cases).ZRevRangeWithScores())
}

func TestZRevRangeByLex(t *testing.T) {
	runCase(t, new(redis.Cases).ZRevRangeByLex())
}

func TestZRevRangeByScore(t *testing.T) {
	runCase(t, new(redis.Cases).ZRevRangeByScore())
}

func TestZRevRank(t *testing.T) {
	runCase(t, new(redis.Cases).ZRevRank())
}

func TestZScore(t *testing.T) {
	runCase(t, new(redis.Cases).ZScore())
}

func TestZUnion(t *testing.T) {
	runCase(t, new(redis.Cases).ZUnion())
}

func TestZUnionStore(t *testing.T) {
	runCase(t, new(redis.Cases).ZUnionStore())
}
