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

func TestHDel(t *testing.T) {
	RunCase(t, testcases.HDel)
}

func TestHExists(t *testing.T) {
	RunCase(t, testcases.HExists)
}

func TestHGet(t *testing.T) {
	RunCase(t, testcases.HGet)
}

func TestHGetAll(t *testing.T) {
	RunCase(t, testcases.HGetAll)
}

func TestHIncrBy(t *testing.T) {
	RunCase(t, testcases.HIncrBy)
}

func TestHIncrByFloat(t *testing.T) {
	RunCase(t, testcases.HIncrByFloat)
}

func TestHKeys(t *testing.T) {
	RunCase(t, testcases.HKeys)
}

func TestHLen(t *testing.T) {
	RunCase(t, testcases.HLen)
}

func TestHMGet(t *testing.T) {
	RunCase(t, testcases.HMGet)
}

func TestHMSet(t *testing.T) {
	RunCase(t, testcases.HMSet)
}

func TestHSet(t *testing.T) {
	RunCase(t, testcases.HSet)
}

func TestHSetNX(t *testing.T) {
	RunCase(t, testcases.HSetNX)
}

func TestHStrLen(t *testing.T) {
	RunCase(t, testcases.HStrLen)
}

func TestHVals(t *testing.T) {
	RunCase(t, testcases.HVals)
}
