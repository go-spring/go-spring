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
	"github.com/go-spring/spring-core/redis/test/cases"
)

func Append(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Append)
}

func Decr(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Decr)
}

func DecrBy(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.DecrBy)
}

func Get(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Get)
}

func GetDel(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.GetDel)
}

func GetRange(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.GetRange)
}

func GetSet(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.GetSet)
}

func Incr(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Incr)
}

func IncrBy(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.IncrBy)
}

func IncrByFloat(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.IncrByFloat)
}

func MGet(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.MGet)
}

func MSet(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.MSet)
}

func MSetNX(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.MSetNX)
}

func PSetEX(t *testing.T, d redis.Driver) {
	// RunCase(t, d, cases.PSetEX)
}

func Set(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Set)
}

func SetEX(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SetEX)
}

func SetNX(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SetNX)
}

func SetRange(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SetRange)
}

func StrLen(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.StrLen)
}
