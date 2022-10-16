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

func TestSAdd(t *testing.T) {
	runCase(t, new(redis.Cases).SAdd())
}

func TestSCard(t *testing.T) {
	runCase(t, new(redis.Cases).SCard())
}

func TestSDiff(t *testing.T) {
	runCase(t, new(redis.Cases).SDiff())
}

func TestSDiffStore(t *testing.T) {
	runCase(t, new(redis.Cases).SDiffStore())
}

func TestSInter(t *testing.T) {
	runCase(t, new(redis.Cases).SInter())
}

func TestSInterStore(t *testing.T) {
	runCase(t, new(redis.Cases).SInterStore())
}

func TestSIsMember(t *testing.T) {
	runCase(t, new(redis.Cases).SIsMember())
}

func TestSMembers(t *testing.T) {
	runCase(t, new(redis.Cases).SMembers())
}

func TestSMIsMember(t *testing.T) {
	runCase(t, new(redis.Cases).SMIsMember())
}

func TestSMove(t *testing.T) {
	runCase(t, new(redis.Cases).SMove())
}

func TestSPop(t *testing.T) {
	runCase(t, new(redis.Cases).SPop())
}

func TestSPopN(t *testing.T) {
	runCase(t, new(redis.Cases).SPopN())
}

func TestSRandMember(t *testing.T) {
	runCase(t, new(redis.Cases).SRandMember())
}

func TestSRem(t *testing.T) {
	runCase(t, new(redis.Cases).SRem())
}

func TestSUnion(t *testing.T) {
	runCase(t, new(redis.Cases).SUnion())
}

func TestSUnionStore(t *testing.T) {
	runCase(t, new(redis.Cases).SUnionStore())
}
