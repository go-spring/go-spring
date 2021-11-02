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

func TestDel(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET key1 Hello","response":"OK"},{"protocol":"redis","request":"SET key2 World","response":"OK"},{"protocol":"redis","request":"DEL key1 key2 key3","response":"2"}]}`
	RunCase(t, testcases.Del, str)
}

func TestDump(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey 10","response":"OK"},{"protocol":"redis","request":"DUMP mykey","response":"\u0000\ufffd\n\t\u0000\ufffdm\u0006\ufffdZ(\u0000\n"}]}`
	RunCase(t, testcases.Dump, str)
}

func TestExists(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET key1 Hello","response":"OK"},{"protocol":"redis","request":"EXISTS key1","response":"1"},{"protocol":"redis","request":"EXISTS nosuchkey","response":"0"},{"protocol":"redis","request":"SET key2 World","response":"OK"},{"protocol":"redis","request":"EXISTS key1 key2 nosuchkey","response":"2"}]}`
	RunCase(t, testcases.Exists, str)
}

func TestExpire(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey Hello","response":"OK"},{"protocol":"redis","request":"EXPIRE mykey 10","response":"1"},{"protocol":"redis","request":"TTL mykey","response":"10"},{"protocol":"redis","request":"SET mykey \"Hello World\"","response":"OK"},{"protocol":"redis","request":"TTL mykey","response":"-1"}]}`
	RunCase(t, testcases.Expire, str)
}

func TestExpireAt(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey Hello","response":"OK"},{"protocol":"redis","request":"EXISTS mykey","response":"1"},{"protocol":"redis","request":"EXPIREAT mykey 1293840000","response":"1"},{"protocol":"redis","request":"EXISTS mykey","response":"0"}]}`
	RunCase(t, testcases.ExpireAt, str)
}

func TestKeys(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.Keys, str)
}

func TestPersist(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey Hello","response":"OK"},{"protocol":"redis","request":"EXPIRE mykey 10","response":"1"},{"protocol":"redis","request":"TTL mykey","response":"10"},{"protocol":"redis","request":"PERSIST mykey","response":"1"},{"protocol":"redis","request":"TTL mykey","response":"-1"}]}`
	RunCase(t, testcases.Persist, str)
}

func TestPExpire(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.PExpire, str)
}

func TestPExpireAt(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey Hello","response":"OK"},{"protocol":"redis","request":"PEXPIREAT mykey 1555555555005","response":"1"},{"protocol":"redis","request":"TTL mykey","response":"-2"},{"protocol":"redis","request":"PTTL mykey","response":"-2"}]}`
	RunCase(t, testcases.PExpireAt, str)
}

func TestPTTL(t *testing.T) {
	str := `skip`
	RunCase(t, testcases.PTTL, str)
}

func TestRename(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey Hello","response":"OK"},{"protocol":"redis","request":"RENAME mykey myotherkey","response":"OK"},{"protocol":"redis","request":"GET myotherkey","response":"Hello"}]}`
	RunCase(t, testcases.Rename, str)
}

func TestRenameNX(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey Hello","response":"OK"},{"protocol":"redis","request":"SET myotherkey World","response":"OK"},{"protocol":"redis","request":"RENAMENX mykey myotherkey","response":"0"},{"protocol":"redis","request":"GET myotherkey","response":"World"}]}`
	RunCase(t, testcases.RenameNX, str)
}

func TestTouch(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET key1 Hello","response":"OK"},{"protocol":"redis","request":"SET key2 World","response":"OK"},{"protocol":"redis","request":"TOUCH key1 key2","response":"2"}]}`
	RunCase(t, testcases.Touch, str)
}

func TestTTL(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET mykey Hello","response":"OK"},{"protocol":"redis","request":"EXPIRE mykey 10","response":"1"},{"protocol":"redis","request":"TTL mykey","response":"10"}]}`
	RunCase(t, testcases.TTL, str)
}

func TestType(t *testing.T) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"SET key1 value","response":"OK"},{"protocol":"redis","request":"LPUSH key2 value","response":"1"},{"protocol":"redis","request":"SADD key3 value","response":"1"},{"protocol":"redis","request":"TYPE key1","response":"string"},{"protocol":"redis","request":"TYPE key2","response":"list"},{"protocol":"redis","request":"TYPE key3","response":"set"}]}`
	RunCase(t, testcases.Type, str)
}
