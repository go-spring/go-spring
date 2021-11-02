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
	str := `skip`
	RunCase(t, testcases.SAdd, str)
}

func TestSCard(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD myset Hello","response":"1"},{"protocol":"redis","request":"SADD myset World","response":"1"},{"protocol":"redis","request":"SCARD myset","response":"2"}]}`
	RunCase(t, testcases.SCard, str)
}

func TestSDiff(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SDiff, str)
}

func TestSDiffStore(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SDiffStore, str)
}

func TestSInter(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":"1"},{"protocol":"redis","request":"SADD key1 b","response":"1"},{"protocol":"redis","request":"SADD key1 c","response":"1"},{"protocol":"redis","request":"SADD key2 c","response":"1"},{"protocol":"redis","request":"SADD key2 d","response":"1"},{"protocol":"redis","request":"SADD key2 e","response":"1"},{"protocol":"redis","request":"SINTER key1 key2","response":"[\"c\"]"}]}`
	RunCase(t, testcases.SInter, str)
}

func TestSInterStore(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":"1"},{"protocol":"redis","request":"SADD key1 b","response":"1"},{"protocol":"redis","request":"SADD key1 c","response":"1"},{"protocol":"redis","request":"SADD key2 c","response":"1"},{"protocol":"redis","request":"SADD key2 d","response":"1"},{"protocol":"redis","request":"SADD key2 e","response":"1"},{"protocol":"redis","request":"SINTERSTORE key key1 key2","response":"1"},{"protocol":"redis","request":"SMEMBERS key","response":"[\"c\"]"}]}`
	RunCase(t, testcases.SInterStore, str)
}

func TestSMembers(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SMembers, str)
}

func TestSMIsMember(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD myset one","response":"1"},{"protocol":"redis","request":"SADD myset one","response":"0"},{"protocol":"redis","request":"SMISMEMBER myset one notamember","response":"[1,0]"}]}`
	RunCase(t, testcases.SMIsMember, str)
}

func TestSMove(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SMove, str)
}

func TestSPop(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SPop, str)
}

func TestSRandMember(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SRandMember, str)
}

func TestSRem(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SRem, str)
}

func TestSUnion(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SUnion, str)
}

func TestSUnionStore(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.SUnionStore, str)
}
