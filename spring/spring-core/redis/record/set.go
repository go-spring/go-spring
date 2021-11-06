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
)

func SAdd(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD myset Hello","response":1},{"protocol":"redis","request":"SADD myset World","response":1},{"protocol":"redis","request":"SADD myset World","response":0},{"protocol":"redis","request":"SMEMBERS myset","response":["Hello","World"]}]}`
	RunCase(t, c, testcases.SAdd, str)
}

func SCard(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD myset Hello","response":1},{"protocol":"redis","request":"SADD myset World","response":1},{"protocol":"redis","request":"SCARD myset","response":2}]}`
	RunCase(t, c, testcases.SCard, str)
}

func SDiff(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":1},{"protocol":"redis","request":"SADD key1 b","response":1},{"protocol":"redis","request":"SADD key1 c","response":1},{"protocol":"redis","request":"SADD key2 c","response":1},{"protocol":"redis","request":"SADD key2 d","response":1},{"protocol":"redis","request":"SADD key2 e","response":1},{"protocol":"redis","request":"SDIFF key1 key2","response":["a","b"]}]}`
	RunCase(t, c, testcases.SDiff, str)
}

func SDiffStore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":1},{"protocol":"redis","request":"SADD key1 b","response":1},{"protocol":"redis","request":"SADD key1 c","response":1},{"protocol":"redis","request":"SADD key2 c","response":1},{"protocol":"redis","request":"SADD key2 d","response":1},{"protocol":"redis","request":"SADD key2 e","response":1},{"protocol":"redis","request":"SDIFFSTORE key key1 key2","response":2},{"protocol":"redis","request":"SMEMBERS key","response":["a","b"]}]}`
	RunCase(t, c, testcases.SDiffStore, str)
}

func SInter(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":1},{"protocol":"redis","request":"SADD key1 b","response":1},{"protocol":"redis","request":"SADD key1 c","response":1},{"protocol":"redis","request":"SADD key2 c","response":1},{"protocol":"redis","request":"SADD key2 d","response":1},{"protocol":"redis","request":"SADD key2 e","response":1},{"protocol":"redis","request":"SINTER key1 key2","response":["c"]}]}`
	RunCase(t, c, testcases.SInter, str)
}

func SInterStore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":1},{"protocol":"redis","request":"SADD key1 b","response":1},{"protocol":"redis","request":"SADD key1 c","response":1},{"protocol":"redis","request":"SADD key2 c","response":1},{"protocol":"redis","request":"SADD key2 d","response":1},{"protocol":"redis","request":"SADD key2 e","response":1},{"protocol":"redis","request":"SINTERSTORE key key1 key2","response":1},{"protocol":"redis","request":"SMEMBERS key","response":["c"]}]}`
	RunCase(t, c, testcases.SInterStore, str)
}

func SMembers(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD myset Hello","response":1},{"protocol":"redis","request":"SADD myset World","response":1},{"protocol":"redis","request":"SMEMBERS myset","response":["Hello","World"]}]}`
	RunCase(t, c, testcases.SMembers, str)
}

func SMIsMember(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD myset one","response":1},{"protocol":"redis","request":"SADD myset one","response":0},{"protocol":"redis","request":"SMISMEMBER myset one notamember","response":[1,0]}]}`
	RunCase(t, c, testcases.SMIsMember, str)
}

func SMove(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD myset one","response":1},{"protocol":"redis","request":"SADD myset two","response":1},{"protocol":"redis","request":"SADD myotherset three","response":1},{"protocol":"redis","request":"SMOVE myset myotherset two","response":1},{"protocol":"redis","request":"SMEMBERS myset","response":["one"]},{"protocol":"redis","request":"SMEMBERS myotherset","response":["three","two"]}]}`
	RunCase(t, c, testcases.SMove, str)
}

func SPop(t *testing.T, c redis.Client) {
	str := `skip`
	RunCase(t, c, testcases.SPop, str)
}

func SRandMember(t *testing.T, c redis.Client) {
	str := `skip`
	RunCase(t, c, testcases.SRandMember, str)
}

func SRem(t *testing.T, c redis.Client) {
	str := `skip`
	RunCase(t, c, testcases.SRem, str)
}

func SUnion(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":1},{"protocol":"redis","request":"SADD key1 b","response":1},{"protocol":"redis","request":"SADD key1 c","response":1},{"protocol":"redis","request":"SADD key2 c","response":1},{"protocol":"redis","request":"SADD key2 d","response":1},{"protocol":"redis","request":"SADD key2 e","response":1},{"protocol":"redis","request":"SUNION key1 key2","response":["a","b","c","e","d"]}]}`
	RunCase(t, c, testcases.SUnion, str)
}

func SUnionStore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SADD key1 a","response":1},{"protocol":"redis","request":"SADD key1 b","response":1},{"protocol":"redis","request":"SADD key1 c","response":1},{"protocol":"redis","request":"SADD key2 c","response":1},{"protocol":"redis","request":"SADD key2 d","response":1},{"protocol":"redis","request":"SADD key2 e","response":1},{"protocol":"redis","request":"SUNIONSTORE key key1 key2","response":5},{"protocol":"redis","request":"SMEMBERS key","response":["a","b","c","e","d"]}]}`
	RunCase(t, c, testcases.SUnionStore, str)
}
