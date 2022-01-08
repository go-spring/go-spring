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

func TestDel(t *testing.T) {
	RunCase(t, cases.Del)
}

func TestDump(t *testing.T) {
	RunCase(t, cases.Dump)
}

func TestExists(t *testing.T) {
	RunCase(t, cases.Exists)
}

func TestExpire(t *testing.T) {
	RunCase(t, cases.Expire)
}

func TestExpireAt(t *testing.T) {
	RunCase(t, cases.ExpireAt)
}

func TestKeys(t *testing.T) {
	RunCase(t, cases.Keys)
}

func TestPersist(t *testing.T) {
	RunCase(t, cases.Persist)
}

func TestPExpire(t *testing.T) {
	RunCase(t, cases.PExpire)
}

func TestPExpireAt(t *testing.T) {
	RunCase(t, cases.PExpireAt)
}

func TestPTTL(t *testing.T) {
	RunCase(t, cases.PTTL)
}

func TestRename(t *testing.T) {
	RunCase(t, cases.Rename)
}

func TestRenameNX(t *testing.T) {
	RunCase(t, cases.RenameNX)
}

func TestTouch(t *testing.T) {
	RunCase(t, cases.Touch)
}

func TestTTL(t *testing.T) {
	RunCase(t, cases.TTL)
}

func TestType(t *testing.T) {
	RunCase(t, cases.Type)
}
