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

package SpringGoRedis_test

import (
	"testing"

	"github.com/go-spring/spring-core/redis"
)

func TestDel(t *testing.T) {
	runCase(t, new(redis.Cases).Del())
}

func TestDump(t *testing.T) {
	runCase(t, new(redis.Cases).Dump())
}

func TestExists(t *testing.T) {
	runCase(t, new(redis.Cases).Exists())
}

func TestExpire(t *testing.T) {
	runCase(t, new(redis.Cases).Expire())
}

func TestExpireAt(t *testing.T) {
	runCase(t, new(redis.Cases).ExpireAt())
}

func TestKeys(t *testing.T) {
	runCase(t, new(redis.Cases).Keys())
}

func TestPersist(t *testing.T) {
	runCase(t, new(redis.Cases).Persist())
}

func TestPExpire(t *testing.T) {
	runCase(t, new(redis.Cases).PExpire())
}

func TestPExpireAt(t *testing.T) {
	runCase(t, new(redis.Cases).PExpireAt())
}

func TestPTTL(t *testing.T) {
	runCase(t, new(redis.Cases).PTTL())
}

func TestRename(t *testing.T) {
	runCase(t, new(redis.Cases).Rename())
}

func TestRenameNX(t *testing.T) {
	runCase(t, new(redis.Cases).RenameNX())
}

func TestTouch(t *testing.T) {
	runCase(t, new(redis.Cases).Touch())
}

func TestTTL(t *testing.T) {
	runCase(t, new(redis.Cases).TTL())
}

func TestType(t *testing.T) {
	runCase(t, new(redis.Cases).Type())
}
