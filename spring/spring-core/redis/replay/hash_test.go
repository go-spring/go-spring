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

func TestHDel(t *testing.T) {
	RunCase(t, testcases.HDel, testdata.HDel)
}

func TestHExists(t *testing.T) {
	RunCase(t, testcases.HExists, testdata.HExists)
}

func TestHGet(t *testing.T) {
	RunCase(t, testcases.HGet, testdata.HGet)
}

func TestHGetAll(t *testing.T) {
	RunCase(t, testcases.HGetAll, testdata.HGetAll)
}

func TestHIncrBy(t *testing.T) {
	RunCase(t, testcases.HIncrBy, testdata.HIncrBy)
}

func TestHIncrByFloat(t *testing.T) {
	RunCase(t, testcases.HIncrByFloat, testdata.HIncrByFloat)
}

func TestHKeys(t *testing.T) {
	RunCase(t, testcases.HKeys, testdata.HKeys)
}

func TestHLen(t *testing.T) {
	RunCase(t, testcases.HLen, testdata.HLen)
}

func TestHMGet(t *testing.T) {
	RunCase(t, testcases.HMGet, testdata.HMGet)
}

func TestHSet(t *testing.T) {
	RunCase(t, testcases.HSet, testdata.HSet)
}

func TestHSetNX(t *testing.T) {
	RunCase(t, testcases.HSetNX, testdata.HSetNX)
}

func TestHStrLen(t *testing.T) {
	RunCase(t, testcases.HStrLen, testdata.HStrLen)
}

func TestHVals(t *testing.T) {
	RunCase(t, testcases.HVals, testdata.HVals)
}
