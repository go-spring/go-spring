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

func Append(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.Append)
}

func Decr(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.Decr)
}

func DecrBy(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.DecrBy)
}

func Get(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.Get)
}

func GetDel(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.GetDel)
}

func GetRange(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.GetRange)
}

func GetSet(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.GetSet)
}

func Incr(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.Incr)
}

func IncrBy(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.IncrBy)
}

func IncrByFloat(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.IncrByFloat)
}

func MGet(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.MGet)
}

func MSet(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.MSet)
}

func MSetNX(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.MSetNX)
}

func PSetEX(t *testing.T, c redis.Client) {
	// RunCase(t, c, cases.PSetEX)
}

func Set(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.Set)
}

func SetEX(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.SetEX)
}

func SetNX(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.SetNX)
}

func SetRange(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.SetRange)
}

func StrLen(t *testing.T, c redis.Client) {
	RunCase(t, c, cases.StrLen)
}
