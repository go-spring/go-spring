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

func SAdd(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SAdd)
}

func SCard(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SCard)
}

func SDiff(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SDiff)
}

func SDiffStore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SDiffStore)
}

func SInter(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SInter)
}

func SInterStore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SInterStore)
}

func SMembers(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SMembers)
}

func SMIsMember(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SMIsMember)
}

func SMove(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SMove)
}

func SPop(t *testing.T, conn redis.ConnPool) {
	// RunCase(t, conn, cases.SPop)
}

func SRandMember(t *testing.T, conn redis.ConnPool) {
	//RunCase(t, conn, cases.SRandMember)
}

func SRem(t *testing.T, conn redis.ConnPool) {
	// RunCase(t, conn, cases.SRem)
}

func SUnion(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SUnion)
}

func SUnionStore(t *testing.T, conn redis.ConnPool) {
	RunCase(t, conn, cases.SUnionStore)
}
