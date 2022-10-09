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

package SpringRedigo_test

import (
	"testing"

	"github.com/go-spring/spring-core/redis"
)

func TestLIndex(t *testing.T) {
	runCase(t, new(redis.Cases).LIndex())
}

func TestLInsert(t *testing.T) {
	runCase(t, new(redis.Cases).LInsert())
}

func TestLLen(t *testing.T) {
	runCase(t, new(redis.Cases).LLen())
}

func TestLMove(t *testing.T) {
	runCase(t, new(redis.Cases).LMove())
}

func TestLPop(t *testing.T) {
	runCase(t, new(redis.Cases).LPop())
}

func TestLPos(t *testing.T) {
	runCase(t, new(redis.Cases).LPos())
}

func TestLPush(t *testing.T) {
	runCase(t, new(redis.Cases).LPush())
}

func TestLPushX(t *testing.T) {
	runCase(t, new(redis.Cases).LPushX())
}

func TestLRange(t *testing.T) {
	runCase(t, new(redis.Cases).LRange())
}

func TestLRem(t *testing.T) {
	runCase(t, new(redis.Cases).LRem())
}

func TestLSet(t *testing.T) {
	runCase(t, new(redis.Cases).LSet())
}

func TestLTrim(t *testing.T) {
	runCase(t, new(redis.Cases).LTrim())
}

func TestRPop(t *testing.T) {
	runCase(t, new(redis.Cases).RPop())
}

func TestRPopLPush(t *testing.T) {
	runCase(t, new(redis.Cases).RPopLPush())
}

func TestRPush(t *testing.T) {
	runCase(t, new(redis.Cases).RPush())
}

func TestRPushX(t *testing.T) {
	runCase(t, new(redis.Cases).RPushX())
}
