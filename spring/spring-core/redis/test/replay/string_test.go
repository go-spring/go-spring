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

func TestAppend(t *testing.T) {
	RunCase(t, cases.Append)
}

func TestDecr(t *testing.T) {
	RunCase(t, cases.Decr)
}

func TestDecrBy(t *testing.T) {
	RunCase(t, cases.DecrBy)
}

func TestGet(t *testing.T) {
	RunCase(t, cases.Get)
}

func TestGetDel(t *testing.T) {
	RunCase(t, cases.GetDel)
}

func TestGetRange(t *testing.T) {
	RunCase(t, cases.GetRange)
}

func TestGetSet(t *testing.T) {
	RunCase(t, cases.GetSet)
}

func TestIncr(t *testing.T) {
	RunCase(t, cases.Incr)
}

func TestIncrBy(t *testing.T) {
	RunCase(t, cases.IncrBy)
}

func TestIncrByFloat(t *testing.T) {
	RunCase(t, cases.IncrByFloat)
}

func TestMGet(t *testing.T) {
	RunCase(t, cases.MGet)
}

func TestMSet(t *testing.T) {
	RunCase(t, cases.MSet)
}

func TestMSetNX(t *testing.T) {
	RunCase(t, cases.MSetNX)
}

func TestPSetEX(t *testing.T) {
	RunCase(t, cases.PSetEX)
}

func TestSet(t *testing.T) {
	RunCase(t, cases.Set)
}

func TestSetEX(t *testing.T) {
	RunCase(t, cases.SetEX)
}

func TestSetNX(t *testing.T) {
	RunCase(t, cases.SetNX)
}

func TestSetRange(t *testing.T) {
	RunCase(t, cases.SetRange)
}

func TestStrLen(t *testing.T) {
	RunCase(t, cases.StrLen)
}
