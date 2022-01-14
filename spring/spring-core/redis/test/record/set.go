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

func SAdd(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SAdd)
}

func SCard(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SCard)
}

func SDiff(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SDiff)
}

func SDiffStore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SDiffStore)
}

func SInter(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SInter)
}

func SInterStore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SInterStore)
}

func SMembers(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SMembers)
}

func SMIsMember(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SMIsMember)
}

func SMove(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SMove)
}

func SPop(t *testing.T, d redis.Driver) {
	// RunCase(t, d, cases.SPop)
}

func SRandMember(t *testing.T, d redis.Driver) {
	//RunCase(t, d, cases.SRandMember)
}

func SRem(t *testing.T, d redis.Driver) {
	// RunCase(t, d, cases.SRem)
}

func SUnion(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SUnion)
}

func SUnionStore(t *testing.T, d redis.Driver) {
	RunCase(t, d, cases.SUnionStore)
}
