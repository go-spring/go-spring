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

func TestAppend(t *testing.T) {
	RunCase(t, testcases.Append, testdata.Append)
}

func TestDecr(t *testing.T) {
	RunCase(t, testcases.Decr, testdata.Decr)
}

func TestDecrBy(t *testing.T) {
	RunCase(t, testcases.DecrBy, testdata.DecrBy)
}

func TestGet(t *testing.T) {
	RunCase(t, testcases.Get, testdata.Get)
}

func TestGetDel(t *testing.T) {
	RunCase(t, testcases.GetDel, testdata.GetDel)
}

func TestGetRange(t *testing.T) {
	RunCase(t, testcases.GetRange, testdata.GetRange)
}

func TestGetSet(t *testing.T) {
	RunCase(t, testcases.GetSet, testdata.GetSet)
}

func TestIncr(t *testing.T) {
	RunCase(t, testcases.Incr, testdata.Incr)
}

func TestIncrBy(t *testing.T) {
	RunCase(t, testcases.IncrBy, testdata.IncrBy)
}

func TestIncrByFloat(t *testing.T) {
	RunCase(t, testcases.IncrByFloat, testdata.IncrByFloat)
}

func TestMGet(t *testing.T) {
	RunCase(t, testcases.MGet, testdata.MGet)
}

func TestMSet(t *testing.T) {
	RunCase(t, testcases.MSet, testdata.MSet)
}

func TestMSetNX(t *testing.T) {
	RunCase(t, testcases.MSetNX, testdata.MSetNX)
}

func TestPSetEX(t *testing.T) {
	RunCase(t, testcases.PSetEX, testdata.PSetEX)
}

func TestSet(t *testing.T) {
	RunCase(t, testcases.Set, testdata.Set)
}

func TestSetEX(t *testing.T) {
	RunCase(t, testcases.SetEX, testdata.SetEX)
}

func TestSetNX(t *testing.T) {
	RunCase(t, testcases.SetNX, testdata.SetNX)
}

func TestSetRange(t *testing.T) {
	RunCase(t, testcases.SetRange, testdata.SetRange)
}

func TestStrLen(t *testing.T) {
	RunCase(t, testcases.StrLen, testdata.StrLen)
}
