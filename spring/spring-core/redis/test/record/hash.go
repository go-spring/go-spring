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

func HDel(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HDel)
}

func HExists(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HExists)
}

func HGet(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HGet)
}

func HGetAll(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HGetAll)
}

func HIncrBy(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HIncrBy)
}

func HIncrByFloat(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HIncrByFloat)
}

func HKeys(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HKeys)
}

func HLen(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HLen)
}

func HMGet(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HMGet)
}

func HSet(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HSet)
}

func HSetNX(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HSetNX)
}

func HStrLen(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HStrLen)
}

func HVals(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.HVals)
}
