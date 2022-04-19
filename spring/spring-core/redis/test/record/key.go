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

func Del(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Del)
}

func Dump(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Dump)
}

func Exists(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Exists)
}

func Expire(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Expire)
}

func ExpireAt(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.ExpireAt)
}

func Keys(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Keys)
}

func Persist(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Persist)
}

func PExpire(t *testing.T, conn redis.ConnPool) {
	// RunCase(t, conn, cases.PExpire)
}

func PExpireAt(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.PExpireAt)
}

func PTTL(t *testing.T, conn redis.ConnPool) {
	// RunCase(t, conn, cases.PTTL)
}

func Rename(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Rename)
}

func RenameNX(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.RenameNX)
}

func Touch(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Touch)
}

func TTL(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.TTL)
}

func Type(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.Type)
}
