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

	"github.com/go-spring/spring-core/redis/testcases"
	"github.com/go-spring/spring-core/redis/testdata"
)

func TestLIndex(t *testing.T) {
	RunCase(t, testcases.LIndex, testdata.LIndex)
}

func TestLInsert(t *testing.T) {
	RunCase(t, testcases.LInsert, testdata.LInsert)
}

func TestLLen(t *testing.T) {
	RunCase(t, testcases.LLen, testdata.LLen)
}

func TestLMove(t *testing.T) {
	RunCase(t, testcases.LMove, testdata.LMove)
}

func TestLPop(t *testing.T) {
	RunCase(t, testcases.LPop, testdata.LPop)
}

func TestLPos(t *testing.T) {
	RunCase(t, testcases.LPos, testdata.LPos)
}

func TestLPush(t *testing.T) {
	RunCase(t, testcases.LPush, testdata.LPush)
}

func TestLPushX(t *testing.T) {
	RunCase(t, testcases.LPushX, testdata.LPushX)
}

func TestLRange(t *testing.T) {
	RunCase(t, testcases.LRange, testdata.LRange)
}

func TestLRem(t *testing.T) {
	RunCase(t, testcases.LRem, testdata.LRem)
}

func TestLSet(t *testing.T) {
	RunCase(t, testcases.LSet, testdata.LSet)
}

func TestLTrim(t *testing.T) {
	RunCase(t, testcases.LTrim, testdata.LTrim)
}

func TestRPop(t *testing.T) {
	RunCase(t, testcases.RPop, testdata.RPop)
}

func TestRPopLPush(t *testing.T) {
	RunCase(t, testcases.RPopLPush, testdata.RPopLPush)
}

func TestRPush(t *testing.T) {
	RunCase(t, testcases.RPush, testdata.RPush)
}

func TestRPushX(t *testing.T) {
	RunCase(t, testcases.RPushX, testdata.RPushX)
}
