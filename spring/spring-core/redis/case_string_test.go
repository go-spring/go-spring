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

func TestAppend(t *testing.T) {
	runCase(t, new(redis.Cases).Append())
}

func TestDecr(t *testing.T) {
	runCase(t, new(redis.Cases).Decr())
}

func TestDecrBy(t *testing.T) {
	runCase(t, new(redis.Cases).DecrBy())
}

func TestGet(t *testing.T) {
	runCase(t, new(redis.Cases).Get())
}

func TestGetDel(t *testing.T) {
	runCase(t, new(redis.Cases).GetDel())
}

func TestGetEx(t *testing.T) {
	runCase(t, new(redis.Cases).GetEx())
}

func TestGetRange(t *testing.T) {
	runCase(t, new(redis.Cases).GetRange())
}

func TestGetSet(t *testing.T) {
	runCase(t, new(redis.Cases).GetSet())
}

func TestIncr(t *testing.T) {
	runCase(t, new(redis.Cases).Incr())
}

func TestIncrBy(t *testing.T) {
	runCase(t, new(redis.Cases).IncrBy())
}

func TestIncrByFloat(t *testing.T) {
	runCase(t, new(redis.Cases).IncrByFloat())
}

func TestMGet(t *testing.T) {
	runCase(t, new(redis.Cases).MGet())
}

func TestMSet(t *testing.T) {
	runCase(t, new(redis.Cases).MSet())
}

func TestMSetNX(t *testing.T) {
	runCase(t, new(redis.Cases).MSetNX())
}

func TestPSetEX(t *testing.T) {
	runCase(t, new(redis.Cases).PSetEX())
}

func TestSet(t *testing.T) {
	runCase(t, new(redis.Cases).Set())
}

func TestSetEX(t *testing.T) {
	runCase(t, new(redis.Cases).SetEX())
}

func TestSetNX(t *testing.T) {
	runCase(t, new(redis.Cases).SetNX())
}

func TestSetRange(t *testing.T) {
	runCase(t, new(redis.Cases).SetRange())
}

func TestStrLen(t *testing.T) {
	runCase(t, new(redis.Cases).StrLen())
}
