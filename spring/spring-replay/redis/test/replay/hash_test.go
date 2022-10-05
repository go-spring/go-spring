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

func TestHDel(t *testing.T) {
	RunCase(t, cases.HDel)
}

func TestHExists(t *testing.T) {
	RunCase(t, cases.HExists)
}

func TestHGet(t *testing.T) {
	RunCase(t, cases.HGet)
}

func TestHGetAll(t *testing.T) {
	RunCase(t, cases.HGetAll)
}

func TestHIncrBy(t *testing.T) {
	RunCase(t, cases.HIncrBy)
}

func TestHIncrByFloat(t *testing.T) {
	RunCase(t, cases.HIncrByFloat)
}

func TestHKeys(t *testing.T) {
	RunCase(t, cases.HKeys)
}

func TestHLen(t *testing.T) {
	RunCase(t, cases.HLen)
}

func TestHMGet(t *testing.T) {
	RunCase(t, cases.HMGet)
}

func TestHSet(t *testing.T) {
	RunCase(t, cases.HSet)
}

func TestHSetNX(t *testing.T) {
	RunCase(t, cases.HSetNX)
}

func TestHStrLen(t *testing.T) {
	RunCase(t, cases.HStrLen)
}

func TestHVals(t *testing.T) {
	RunCase(t, cases.HVals)
}
