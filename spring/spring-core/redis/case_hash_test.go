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

func TestHDel(t *testing.T) {
	runCase(t, new(redis.Cases).HDel())
}

func TestHExists(t *testing.T) {
	runCase(t, new(redis.Cases).HExists())
}

func TestHGet(t *testing.T) {
	runCase(t, new(redis.Cases).HGet())
}

func TestHGetAll(t *testing.T) {
	runCase(t, new(redis.Cases).HGetAll())
}

func TestHIncrBy(t *testing.T) {
	runCase(t, new(redis.Cases).HIncrBy())
}

func TestHIncrByFloat(t *testing.T) {
	runCase(t, new(redis.Cases).HIncrByFloat())
}

func TestHKeys(t *testing.T) {
	runCase(t, new(redis.Cases).HKeys())
}

func TestHLen(t *testing.T) {
	runCase(t, new(redis.Cases).HLen())
}

func TestHMGet(t *testing.T) {
	runCase(t, new(redis.Cases).HMGet())
}

func TestHSet(t *testing.T) {
	runCase(t, new(redis.Cases).HSet())
}

func TestHSetNX(t *testing.T) {
	runCase(t, new(redis.Cases).HSetNX())
}

func TestHStrLen(t *testing.T) {
	runCase(t, new(redis.Cases).HStrLen())
}

func TestHVals(t *testing.T) {
	runCase(t, new(redis.Cases).HVals())
}
