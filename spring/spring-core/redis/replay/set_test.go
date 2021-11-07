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

	"github.com/go-spring/spring-core/redis/testcases"
	"github.com/go-spring/spring-core/redis/testdata"
)

func TestSAdd(t *testing.T) {
	RunCase(t, testcases.SAdd, testdata.SAdd)
}

func TestSCard(t *testing.T) {
	RunCase(t, testcases.SCard, testdata.SCard)
}

func TestSDiff(t *testing.T) {
	RunCase(t, testcases.SDiff, testdata.SDiff)
}

func TestSDiffStore(t *testing.T) {
	RunCase(t, testcases.SDiffStore, testdata.SDiffStore)
}

func TestSInter(t *testing.T) {
	RunCase(t, testcases.SInter, testdata.SInter)
}

func TestSInterStore(t *testing.T) {
	RunCase(t, testcases.SInterStore, testdata.SInterStore)
}

func TestSMembers(t *testing.T) {
	RunCase(t, testcases.SMembers, testdata.SMembers)
}

func TestSMIsMember(t *testing.T) {
	RunCase(t, testcases.SMIsMember, testdata.SMIsMember)
}

func TestSMove(t *testing.T) {
	RunCase(t, testcases.SMove, testdata.SMove)
}

func TestSPop(t *testing.T) {
	RunCase(t, testcases.SPop, testdata.SPop)
}

func TestSRandMember(t *testing.T) {
	RunCase(t, testcases.SRandMember, testdata.SRandMember)
}

func TestSRem(t *testing.T) {
	RunCase(t, testcases.SRem, testdata.SRem)
}

func TestSUnion(t *testing.T) {
	RunCase(t, testcases.SUnion, testdata.SUnion)
}

func TestSUnionStore(t *testing.T) {
	RunCase(t, testcases.SUnionStore, testdata.SUnionStore)
}
