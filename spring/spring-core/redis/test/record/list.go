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

func LIndex(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LIndex)
}

func LInsert(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LInsert)
}

func LLen(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LLen)
}

func LMove(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LMove)
}

func LPop(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LPop)
}

func LPos(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LPos)
}

func LPush(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LPush)
}

func LPushX(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LPushX)
}

func LRange(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LRange)
}

func LRem(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LRem)
}

func LSet(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LSet)
}

func LTrim(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.LTrim)
}

func RPop(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.RPop)
}

func RPopLPush(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.RPopLPush)
}

func RPush(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.RPush)
}

func RPushX(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.RPushX)
}
