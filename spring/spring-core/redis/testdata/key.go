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

package testdata

const Del = `
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

const Dump = `
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
		"response": "@\"\\x00\\xc0\\n\\t\\x00\\xbem\\x06\\x89Z(\\x00\\n\"@"
	}]
}`

const Exists = `
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

const Expire = `
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
		"request": "SET mykey @\"Hello World\"@",
		"response": "OK"
	}, {
		"protocol": "redis",
		"request": "TTL mykey",
		"response": -1
	}]
}`

const ExpireAt = `
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

const Keys = `
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

const Persist = `
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

const PExpire = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SET mykey Hello",
		"response": "OK"
	}, {
		"protocol": "redis",
		"request": "PEXPIRE mykey 1500",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "TTL mykey",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "PTTL mykey",
		"response": 1499
	}]
}`

const PExpireAt = `
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

const PTTL = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SET mykey Hello",
		"response": "OK"
	}, {
		"protocol": "redis",
		"request": "EXPIRE mykey 1",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "PTTL mykey",
		"response": 1000
	}]
}`

const Rename = `
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

const RenameNX = `
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

const Touch = `
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

const TTL = `
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

const Type = `
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
