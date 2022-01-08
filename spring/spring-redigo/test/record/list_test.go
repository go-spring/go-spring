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

	"github.com/go-spring/spring-core/redis/test/record"
)

func TestLIndex(t *testing.T) {
	RunCase(t, record.LIndex)
}

func TestLInsert(t *testing.T) {
	RunCase(t, record.LInsert)
}

func TestLLen(t *testing.T) {
	RunCase(t, record.LLen)
}

func TestLMove(t *testing.T) {
	RunCase(t, record.LMove)
}

func TestLPop(t *testing.T) {
	RunCase(t, record.LPop)
}

func TestLPos(t *testing.T) {
	RunCase(t, record.LPos)
}

func TestLPush(t *testing.T) {
	RunCase(t, record.LPush)
}

func TestLPushX(t *testing.T) {
	RunCase(t, record.LPushX)
}

func TestLRange(t *testing.T) {
	RunCase(t, record.LRange)
}

func TestLRem(t *testing.T) {
	RunCase(t, record.LRem)
}

func TestLSet(t *testing.T) {
	RunCase(t, record.LSet)
}

func TestLTrim(t *testing.T) {
	RunCase(t, record.LTrim)
}

func TestRPop(t *testing.T) {
	RunCase(t, record.RPop)
}

func TestRPopLPush(t *testing.T) {
	RunCase(t, record.RPopLPush)
}

func TestRPush(t *testing.T) {
	RunCase(t, record.RPush)
}

func TestRPushX(t *testing.T) {
	RunCase(t, record.RPushX)
}
