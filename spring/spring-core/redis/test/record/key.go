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

func Del(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Del)
}

func Dump(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Dump)
}

func Exists(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Exists)
}

func Expire(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Expire)
}

func ExpireAt(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.ExpireAt)
}

func Keys(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Keys)
}

func Persist(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Persist)
}

func PExpire(t *testing.T, d redis.Driver) {
	// RunCase(t, d, cases.PExpire)
}

func PExpireAt(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.PExpireAt)
}

func PTTL(t *testing.T, d redis.Driver) {
	// RunCase(t, d, cases.PTTL)
}

func Rename(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Rename)
}

func RenameNX(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.RenameNX)
}

func Touch(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Touch)
}

func TTL(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.TTL)
}

func Type(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.Type)
}
