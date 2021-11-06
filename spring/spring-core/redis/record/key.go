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

func Del(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET key1 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SET key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "DEL key1 key2 key3",
			"response": 2
		}]
	}`
	RunCase(t, c, testcases.Del, str)
}

func Dump(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey 10",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "DUMP mykey",
			"response": "\u0000\ufffd\n\t\u0000\ufffdm\u0006\ufffdZ(\u0000\n"
		}]
	}`
	RunCase(t, c, testcases.Dump, str)
}

func Exists(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET key1 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXISTS key1",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "EXISTS nosuchkey",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "SET key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXISTS key1 key2 nosuchkey",
			"response": 2
		}]
	}`
	RunCase(t, c, testcases.Exists, str)
}

func Expire(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXPIRE mykey 10",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}, {
			"protocol": "redis",
			"request": "SET mykey \"Hello World\"",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": -1
		}]
	}`
	RunCase(t, c, testcases.Expire, str)
}

func ExpireAt(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXISTS mykey",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "EXPIREAT mykey 1293840000",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "EXISTS mykey",
			"response": 0
		}]
	}`
	RunCase(t, c, testcases.ExpireAt, str)
}

func Keys(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "MSET firstname Jack lastname Stuntman age 35",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "KEYS *name*",
			"response": ["lastname", "firstname"]
		}, {
			"protocol": "redis",
			"request": "KEYS a??",
			"response": ["age"]
		}, {
			"protocol": "redis",
			"request": "KEYS *",
			"response": ["age", "lastname", "firstname"]
		}]
	}`
	RunCase(t, c, testcases.Keys, str)
}

func Persist(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXPIRE mykey 10",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}, {
			"protocol": "redis",
			"request": "PERSIST mykey",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": -1
		}]
	}`
	RunCase(t, c, testcases.Persist, str)
}

func PExpire(t *testing.T, c redis.Client) {
	str := `skip`
	RunCase(t, c, testcases.PExpire, str)
}

func PExpireAt(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "PEXPIREAT mykey 1555555555005",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": -2
		}, {
			"protocol": "redis",
			"request": "PTTL mykey",
			"response": -2
		}]
	}`
	RunCase(t, c, testcases.PExpireAt, str)
}

func PTTL(t *testing.T, c redis.Client) {
	str := `skip`
	RunCase(t, c, testcases.PTTL, str)
}

func Rename(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "RENAME mykey myotherkey",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GET myotherkey",
			"response": "Hello"
		}]
	}`
	RunCase(t, c, testcases.Rename, str)
}

func RenameNX(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SET myotherkey World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "RENAMENX mykey myotherkey",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "GET myotherkey",
			"response": "World"
		}]
	}`
	RunCase(t, c, testcases.RenameNX, str)
}

func Touch(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET key1 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SET key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "TOUCH key1 key2",
			"response": 2
		}]
	}`
	RunCase(t, c, testcases.Touch, str)
}

func TTL(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "EXPIRE mykey 10",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}]
	}`
	RunCase(t, c, testcases.TTL, str)
}

func Type(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET key1 value",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "LPUSH key2 value",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SADD key3 value",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "TYPE key1",
			"response": "string"
		}, {
			"protocol": "redis",
			"request": "TYPE key2",
			"response": "list"
		}, {
			"protocol": "redis",
			"request": "TYPE key3",
			"response": "set"
		}]
	}`
	RunCase(t, c, testcases.Type, str)
}
