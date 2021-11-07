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

func Append(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Append, testdata.Append)
}

func Decr(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Decr, testdata.Decr)
}

func DecrBy(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.DecrBy, testdata.DecrBy)
}

func Get(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Get, testdata.Get)
}

func GetDel(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.GetDel, testdata.GetDel)
}

func GetRange(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.GetRange, testdata.GetRange)
}

func GetSet(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.GetSet, testdata.GetSet)
}

func Incr(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Incr, testdata.Incr)
}

func IncrBy(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.IncrBy, testdata.IncrBy)
}

func IncrByFloat(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.IncrByFloat, testdata.IncrByFloat)
}

func MGet(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.MGet, testdata.MGet)
}

func MSet(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.MSet, testdata.MSet)
}

func MSetNX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.MSetNX, testdata.MSetNX)
}

func PSetEX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.PSetEX, "skip")
}

func Set(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Set, testdata.Set)
}

func SetEX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.SetEX, testdata.SetEX)
}

func SetNX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.SetNX, testdata.SetNX)
}

func SetRange(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.SetRange, testdata.SetRange)
}

func StrLen(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.StrLen, testdata.StrLen)
}
