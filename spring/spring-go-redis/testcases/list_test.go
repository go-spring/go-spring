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

func TestLIndex(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"LPUSH mylist World","response":"1"},{"protocol":"redis","request":"LPUSH mylist Hello","response":"2"},{"protocol":"redis","request":"LINDEX mylist 0","response":"Hello"},{"protocol":"redis","request":"LINDEX mylist -1","response":"World"}]}`
	RunCase(t, testcases.LIndex, str)
}

func TestLInsert(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist Hello","response":"1"},{"protocol":"redis","request":"RPUSH mylist World","response":"2"},{"protocol":"redis","request":"LINSERT mylist BEFORE World There","response":"3"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"Hello\",\"There\",\"World\"]"}]}`
	RunCase(t, testcases.LInsert, str)
}

func TestLLen(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"LPUSH mylist World","response":"1"},{"protocol":"redis","request":"LPUSH mylist Hello","response":"2"},{"protocol":"redis","request":"LLEN mylist","response":"2"}]}`
	RunCase(t, testcases.LLen, str)
}

func TestLMove(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist one","response":"1"},{"protocol":"redis","request":"RPUSH mylist two","response":"2"},{"protocol":"redis","request":"RPUSH mylist three","response":"3"},{"protocol":"redis","request":"LMOVE mylist myotherlist RIGHT LEFT","response":"three"},{"protocol":"redis","request":"LMOVE mylist myotherlist LEFT RIGHT","response":"one"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"two\"]"},{"protocol":"redis","request":"LRANGE myotherlist 0 -1","response":"[\"three\",\"one\"]"}]}`
	RunCase(t, testcases.LMove, str)
}

func TestLPop(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist one two three four five","response":"5"},{"protocol":"redis","request":"LPOP mylist","response":"one"},{"protocol":"redis","request":"LPOP mylist 2","response":"[\"two\",\"three\"]"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"four\",\"five\"]"}]}`
	RunCase(t, testcases.LPop, str)
}

func TestLPos(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist 97 98 99 100 1 2 3 4 3 3 3","response":"11"},{"protocol":"redis","request":"LPOS mylist 3","response":"6"},{"protocol":"redis","request":"LPOS mylist 3 COUNT 0 RANK 2","response":"[8,9,10]"}]}`
	RunCase(t, testcases.LPos, str)
}

func TestLPush(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"LPUSH mylist world","response":"1"},{"protocol":"redis","request":"LPUSH mylist hello","response":"2"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"hello\",\"world\"]"}]}`
	RunCase(t, testcases.LPush, str)
}

func TestLPushX(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"LPUSH mylist World","response":"1"},{"protocol":"redis","request":"LPUSHX mylist Hello","response":"2"},{"protocol":"redis","request":"LPUSHX myotherlist Hello","response":"0"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"Hello\",\"World\"]"},{"protocol":"redis","request":"LRANGE myotherlist 0 -1","response":"[]"}]}`
	RunCase(t, testcases.LPushX, str)
}

func TestLRange(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist one","response":"1"},{"protocol":"redis","request":"RPUSH mylist two","response":"2"},{"protocol":"redis","request":"RPUSH mylist three","response":"3"},{"protocol":"redis","request":"LRANGE mylist 0 0","response":"[\"one\"]"},{"protocol":"redis","request":"LRANGE mylist -3 2","response":"[\"one\",\"two\",\"three\"]"},{"protocol":"redis","request":"LRANGE mylist -100 100","response":"[\"one\",\"two\",\"three\"]"},{"protocol":"redis","request":"LRANGE mylist 5 10","response":"[]"}]}`
	RunCase(t, testcases.LRange, str)
}

func TestLRem(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist hello","response":"1"},{"protocol":"redis","request":"RPUSH mylist hello","response":"2"},{"protocol":"redis","request":"RPUSH mylist foo","response":"3"},{"protocol":"redis","request":"RPUSH mylist hello","response":"4"},{"protocol":"redis","request":"LREM mylist -2 hello","response":"2"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"hello\",\"foo\"]"}]}`
	RunCase(t, testcases.LRem, str)
}

func TestLSet(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist one","response":"1"},{"protocol":"redis","request":"RPUSH mylist two","response":"2"},{"protocol":"redis","request":"RPUSH mylist three","response":"3"},{"protocol":"redis","request":"LSET mylist 0 four","response":"OK"},{"protocol":"redis","request":"LSET mylist -2 five","response":"OK"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"four\",\"five\",\"three\"]"}]}`
	RunCase(t, testcases.LSet, str)
}

func TestLTrim(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist one","response":"1"},{"protocol":"redis","request":"RPUSH mylist two","response":"2"},{"protocol":"redis","request":"RPUSH mylist three","response":"3"},{"protocol":"redis","request":"LTRIM mylist 1 -1","response":"OK"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"two\",\"three\"]"}]}`
	RunCase(t, testcases.LTrim, str)
}

func TestRPop(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist one two three four five","response":"5"},{"protocol":"redis","request":"RPOP mylist","response":"five"},{"protocol":"redis","request":"RPOP mylist 2","response":"[\"four\",\"three\"]"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"one\",\"two\"]"}]}`
	RunCase(t, testcases.RPop, str)
}

func TestRPopLPush(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist one","response":"1"},{"protocol":"redis","request":"RPUSH mylist two","response":"2"},{"protocol":"redis","request":"RPUSH mylist three","response":"3"},{"protocol":"redis","request":"RPOPLPUSH mylist myotherlist","response":"three"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"one\",\"two\"]"},{"protocol":"redis","request":"LRANGE myotherlist 0 -1","response":"[\"three\"]"}]}`
	RunCase(t, testcases.RPopLPush, str)
}

func TestRPush(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist hello","response":"1"},{"protocol":"redis","request":"RPUSH mylist world","response":"2"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"hello\",\"world\"]"}]}`
	RunCase(t, testcases.RPush, str)
}

func TestRPushX(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"RPUSH mylist Hello","response":"1"},{"protocol":"redis","request":"RPUSHX mylist World","response":"2"},{"protocol":"redis","request":"RPUSHX myotherlist World","response":"0"},{"protocol":"redis","request":"LRANGE mylist 0 -1","response":"[\"Hello\",\"World\"]"},{"protocol":"redis","request":"LRANGE myotherlist 0 -1","response":"[]"}]}`
	RunCase(t, testcases.RPushX, str)
}
