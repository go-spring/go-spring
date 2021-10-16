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

package testcases

import (
	"testing"

	"github.com/go-spring/spring-core/redis/testcases"
)

func TestSAdd(t *testing.T) {
	RunCase(t, testcases.SAdd)
}

func TestSCard(t *testing.T) {
	RunCase(t, testcases.SCard)
}

func TestSDiff(t *testing.T) {
	RunCase(t, testcases.SDiff)
}

func TestSDiffStore(t *testing.T) {
	RunCase(t, testcases.SDiffStore)
}

func TestSInter(t *testing.T) {
	RunCase(t, testcases.SInter)
}

func TestSInterStore(t *testing.T) {
	RunCase(t, testcases.SInterStore)
}

func TestSMembers(t *testing.T) {
	RunCase(t, testcases.SMembers)
}

func TestSMIsMember(t *testing.T) {
	RunCase(t, testcases.SMIsMember)
}

func TestSMove(t *testing.T) {
	RunCase(t, testcases.SMove)
}

func TestSPop(t *testing.T) {
	RunCase(t, testcases.SPop)
}

func TestSRandMember(t *testing.T) {
	RunCase(t, testcases.SRandMember)
}

func TestSRem(t *testing.T) {
	RunCase(t, testcases.SRem)
}

func TestSUnion(t *testing.T) {
	RunCase(t, testcases.SUnion)
}

func TestSUnionStore(t *testing.T) {
	RunCase(t, testcases.SUnionStore)
}
