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

func TestSAdd(t *testing.T) {
	RunCase(t, cases.SAdd)
}

func TestSCard(t *testing.T) {
	RunCase(t, cases.SCard)
}

func TestSDiff(t *testing.T) {
	RunCase(t, cases.SDiff)
}

func TestSDiffStore(t *testing.T) {
	RunCase(t, cases.SDiffStore)
}

func TestSInter(t *testing.T) {
	RunCase(t, cases.SInter)
}

func TestSInterStore(t *testing.T) {
	RunCase(t, cases.SInterStore)
}

func TestSMembers(t *testing.T) {
	RunCase(t, cases.SMembers)
}

func TestSMIsMember(t *testing.T) {
	RunCase(t, cases.SMIsMember)
}

func TestSMove(t *testing.T) {
	RunCase(t, cases.SMove)
}

func TestSPop(t *testing.T) {
	RunCase(t, cases.SPop)
}

func TestSRandMember(t *testing.T) {
	RunCase(t, cases.SRandMember)
}

func TestSRem(t *testing.T) {
	RunCase(t, cases.SRem)
}

func TestSUnion(t *testing.T) {
	RunCase(t, cases.SUnion)
}

func TestSUnionStore(t *testing.T) {
	RunCase(t, cases.SUnionStore)
}
