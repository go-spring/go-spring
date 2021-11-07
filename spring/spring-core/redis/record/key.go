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

func Del(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Del, testdata.Del)
}

func Dump(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Dump, testdata.Dump)
}

func Exists(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Exists, testdata.Exists)
}

func Expire(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Expire, testdata.Expire)
}

func ExpireAt(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.ExpireAt, testdata.ExpireAt)
}

func Keys(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Keys, testdata.Keys)
}

func Persist(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Persist, testdata.Persist)
}

func PExpire(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.PExpire, "skip")
}

func PExpireAt(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.PExpireAt, testdata.PExpireAt)
}

func PTTL(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.PTTL, "skip")
}

func Rename(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Rename, testdata.Rename)
}

func RenameNX(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.RenameNX, testdata.RenameNX)
}

func Touch(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Touch, testdata.Touch)
}

func TTL(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.TTL, testdata.TTL)
}

func Type(t *testing.T, c redis.Client) {
	RunCase(t, c, testcases.Type, testdata.Type)
}
