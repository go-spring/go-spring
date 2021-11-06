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

func Append(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "EXISTS mykey",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "APPEND mykey Hello",
			"response": 5
		}, {
			"protocol": "redis",
			"request": "APPEND mykey \" World\"",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello World"
		}]
	}`
	RunCase(t, c, testcases.Append, str)
}

func Decr(t *testing.T, c redis.Client) {
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
			"request": "DECR mykey",
			"response": 9
		}, {
			"protocol": "redis",
			"request": "SET mykey 234293482390480948029348230948",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "DECR mykey",
			"response": "(err) ERR value is not an integer or out of range"
		}]
	}`
	RunCase(t, c, testcases.Decr, str)
}

func DecrBy(t *testing.T, c redis.Client) {
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
			"request": "DECRBY mykey 3",
			"response": 7
		}]
	}`
	RunCase(t, c, testcases.DecrBy, str)
}

func Get(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "GET nonexisting",
			"response": "(nil)"
		}, {
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello"
		}]
	}`
	RunCase(t, c, testcases.Get, str)
}

func GetDel(t *testing.T, c redis.Client) {
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
			"request": "GETDEL mykey",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "(nil)"
		}]
	}`
	RunCase(t, c, testcases.GetDel, str)
}

func GetRange(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey \"This is a string\"",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey 0 3",
			"response": "This"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey -3 -1",
			"response": "ing"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey 0 -1",
			"response": "This is a string"
		}, {
			"protocol": "redis",
			"request": "GETRANGE mykey 10 100",
			"response": "string"
		}]
	}`
	RunCase(t, c, testcases.GetRange, str)
}

func GetSet(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "INCR mycounter",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "GETSET mycounter 0",
			"response": "1"
		}, {
			"protocol": "redis",
			"request": "GET mycounter",
			"response": "0"
		}, {
			"protocol": "redis",
			"request": "SET mykey Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GETSET mykey World",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "World"
		}]
	}`
	RunCase(t, c, testcases.GetSet, str)
}

func Incr(t *testing.T, c redis.Client) {
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
			"request": "INCR mykey",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "11"
		}]
	}`
	RunCase(t, c, testcases.Incr, str)
}

func IncrBy(t *testing.T, c redis.Client) {
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
			"request": "INCRBY mykey 5",
			"response": 15
		}]
	}`
	RunCase(t, c, testcases.IncrBy, str)
}

func IncrByFloat(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey 10.5",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "INCRBYFLOAT mykey 0.1",
			"response": "10.6"
		}, {
			"protocol": "redis",
			"request": "INCRBYFLOAT mykey -5",
			"response": "5.6"
		}, {
			"protocol": "redis",
			"request": "SET mykey 5000",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "INCRBYFLOAT mykey 200",
			"response": "5200"
		}]
	}`
	RunCase(t, c, testcases.IncrByFloat, str)
}

func MGet(t *testing.T, c redis.Client) {
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
			"request": "MGET key1 key2 nonexisting",
			"response": ["Hello", "World", null]
		}]
	}`
	RunCase(t, c, testcases.MGet, str)
}

func MSet(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "MSET key1 Hello key2 World",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "GET key1",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "GET key2",
			"response": "World"
		}]
	}`
	RunCase(t, c, testcases.MSet, str)
}

func MSetNX(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "MSETNX key1 Hello key2 there",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "MSETNX key2 new key3 world",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "MGET key1 key2 key3",
			"response": ["Hello", "there", null]
		}]
	}`
	RunCase(t, c, testcases.MSetNX, str)
}

func PSetEX(t *testing.T, c redis.Client) {
	str := `skip`
	RunCase(t, c, testcases.PSetEX, str)
}

func Set(t *testing.T, c redis.Client) {
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
			"request": "GET mykey",
			"response": "Hello"
		}, {
			"protocol": "redis",
			"request": "SETEX anotherkey 60 \"will expire in a minute\"",
			"response": "OK"
		}]
	}`
	RunCase(t, c, testcases.Set, str)
}

func SetEX(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SETEX mykey 10 Hello",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "TTL mykey",
			"response": 10
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello"
		}]
	}`
	RunCase(t, c, testcases.SetEX, str)
}

func SetNX(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SETNX mykey Hello",
			"response": 1
		}, {
			"protocol": "redis",
			"request": "SETNX mykey World",
			"response": 0
		}, {
			"protocol": "redis",
			"request": "GET mykey",
			"response": "Hello"
		}]
	}`
	RunCase(t, c, testcases.SetNX, str)
}

func SetRange(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET key1 \"Hello World\"",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "SETRANGE key1 6 Redis",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET key1",
			"response": "Hello Redis"
		}, {
			"protocol": "redis",
			"request": "SETRANGE key2 6 Redis",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "GET key2",
			"response": "\u0000\u0000\u0000\u0000\u0000\u0000Redis"
		}]
	}`
	RunCase(t, c, testcases.SetRange, str)
}

func StrLen(t *testing.T, c redis.Client) {
	str := `
	{
		"session": "df3b64266ebe4e63a464e135000a07cd",
		"inbound": {},
		"actions": [{
			"protocol": "redis",
			"request": "SET mykey \"Hello world\"",
			"response": "OK"
		}, {
			"protocol": "redis",
			"request": "STRLEN mykey",
			"response": 11
		}, {
			"protocol": "redis",
			"request": "STRLEN nonexisting",
			"response": 0
		}]
	}`
	RunCase(t, c, testcases.StrLen, str)
}
