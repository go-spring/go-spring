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
	"github.com/go-spring/spring-core/redis/testcases"
	"github.com/go-spring/spring-core/redis/testdata"
)

func HDel(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HDel, testdata.HDel)
}

func HExists(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HExists, testdata.HExists)
}

func HGet(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HGet, testdata.HGet)
}

func HGetAll(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HGetAll, testdata.HGetAll)
}

func HIncrBy(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HIncrBy, testdata.HIncrBy)
}

func HIncrByFloat(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HIncrByFloat, testdata.HIncrByFloat)
}

func HKeys(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HKeys, testdata.HKeys)
}

func HLen(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HLen, testdata.HLen)
}

func HMGet(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HMGet, testdata.HMGet)
}

func HSet(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HSet, testdata.HSet)
}

func HSetNX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HSetNX, testdata.HSetNX)
}

func HStrLen(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HStrLen, testdata.HStrLen)
}

func HVals(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.HVals, testdata.HVals)
}
