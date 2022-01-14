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

func HDel(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HDel)
}

func HExists(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HExists)
}

func HGet(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HGet)
}

func HGetAll(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HGetAll)
}

func HIncrBy(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HIncrBy)
}

func HIncrByFloat(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HIncrByFloat)
}

func HKeys(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HKeys)
}

func HLen(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HLen)
}

func HMGet(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HMGet)
}

func HSet(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HSet)
}

func HSetNX(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HSetNX)
}

func HStrLen(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HStrLen)
}

func HVals(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.HVals)
}
