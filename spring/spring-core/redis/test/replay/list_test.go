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

package replay

import (
	"testing"

	"github.com/go-spring/spring-core/redis/test/cases"
)

func TestLIndex(t *testing.T) {
	RunCase(t, cases.LIndex)
}

func TestLInsert(t *testing.T) {
	RunCase(t, cases.LInsert)
}

func TestLLen(t *testing.T) {
	RunCase(t, cases.LLen)
}

func TestLMove(t *testing.T) {
	RunCase(t, cases.LMove)
}

func TestLPop(t *testing.T) {
	RunCase(t, cases.LPop)
}

func TestLPos(t *testing.T) {
	RunCase(t, cases.LPos)
}

func TestLPush(t *testing.T) {
	RunCase(t, cases.LPush)
}

func TestLPushX(t *testing.T) {
	RunCase(t, cases.LPushX)
}

func TestLRange(t *testing.T) {
	RunCase(t, cases.LRange)
}

func TestLRem(t *testing.T) {
	RunCase(t, cases.LRem)
}

func TestLSet(t *testing.T) {
	RunCase(t, cases.LSet)
}

func TestLTrim(t *testing.T) {
	RunCase(t, cases.LTrim)
}

func TestRPop(t *testing.T) {
	RunCase(t, cases.RPop)
}

func TestRPopLPush(t *testing.T) {
	RunCase(t, cases.RPopLPush)
}

func TestRPush(t *testing.T) {
	RunCase(t, cases.RPush)
}

func TestRPushX(t *testing.T) {
	RunCase(t, cases.RPushX)
}
