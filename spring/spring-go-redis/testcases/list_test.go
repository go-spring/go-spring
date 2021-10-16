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

package testcases

import (
	"testing"

	"github.com/go-spring/spring-core/redis/testcases"
)

func TestLIndex(t *testing.T) {
	RunCase(t, testcases.LIndex)
}

func TestLInsert(t *testing.T) {
	RunCase(t, testcases.LInsert)
}

func TestLLen(t *testing.T) {
	RunCase(t, testcases.LLen)
}

func TestLMove(t *testing.T) {
	RunCase(t, testcases.LMove)
}

func TestLPop(t *testing.T) {
	RunCase(t, testcases.LPop)
}

func TestLPos(t *testing.T) {
	RunCase(t, testcases.LPos)
}

func TestLPush(t *testing.T) {
	RunCase(t, testcases.LPush)
}

func TestLPushX(t *testing.T) {
	RunCase(t, testcases.LPushX)
}

func TestLRange(t *testing.T) {
	RunCase(t, testcases.LRange)
}

func TestLRem(t *testing.T) {
	RunCase(t, testcases.LRem)
}

func TestLSet(t *testing.T) {
	RunCase(t, testcases.LSet)
}

func TestLTrim(t *testing.T) {
	RunCase(t, testcases.LTrim)
}

func TestRPop(t *testing.T) {
	RunCase(t, testcases.RPop)
}

func TestRPopLPush(t *testing.T) {
	RunCase(t, testcases.RPopLPush)
}

func TestRPush(t *testing.T) {
	RunCase(t, testcases.RPush)
}

func TestRPushX(t *testing.T) {
	RunCase(t, testcases.RPushX)
}
