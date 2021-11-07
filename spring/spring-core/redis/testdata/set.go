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

const SAdd = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset World",
		"response": 0
	}, {
		"protocol": "redis",
		"request": "SMEMBERS myset",
		"response": ["Hello", "World"]
	}]
}`

const SCard = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SCARD myset",
		"response": 2
	}]
}`

const SDiff = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD key1 a",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 b",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 d",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 e",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SDIFF key1 key2",
		"response": ["a", "b"]
	}]
}`

const SDiffStore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD key1 a",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 b",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 d",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 e",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SDIFFSTORE key key1 key2",
		"response": 2
	}, {
		"protocol": "redis",
		"request": "SMEMBERS key",
		"response": ["a", "b"]
	}]
}`

const SInter = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD key1 a",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 b",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 d",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 e",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SINTER key1 key2",
		"response": ["c"]
	}]
}`

const SInterStore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD key1 a",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 b",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 d",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 e",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SINTERSTORE key key1 key2",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SMEMBERS key",
		"response": ["c"]
	}]
}`

const SMembers = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SMEMBERS myset",
		"response": ["Hello", "World"]
	}]
}`

const SMIsMember = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset one",
		"response": 0
	}, {
		"protocol": "redis",
		"request": "SMISMEMBER myset one notamember",
		"response": [1, 0]
	}]
}`

const SMove = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myotherset three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SMOVE myset myotherset two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SMEMBERS myset",
		"response": ["one"]
	}, {
		"protocol": "redis",
		"request": "SMEMBERS myotherset",
		"response": ["three", "two"]
	}]
}`

const SPop = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SPOP myset",
		"response": "two"
	}, {
		"protocol": "redis",
		"request": "SMEMBERS myset",
		"response": ["three", "one"]
	}]
}`

const SRandMember = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset one two three",
		"response": 3
	}, {
		"protocol": "redis",
		"request": "SRANDMEMBER myset",
		"response": "one"
	}, {
		"protocol": "redis",
		"request": "SRANDMEMBER myset 2",
		"response": ["one", "three"]
	}, {
		"protocol": "redis",
		"request": "SRANDMEMBER myset -5",
		"response": ["one", "one", "one", "two", "one"]
	}]
}`

const SRem = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD myset one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD myset three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SREM myset one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SREM myset four",
		"response": 0
	}, {
		"protocol": "redis",
		"request": "SMEMBERS myset",
		"response": ["three", "two"]
	}]
}`

const SUnion = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD key1 a",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 b",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 d",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 e",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SUNION key1 key2",
		"response": ["a", "b", "c", "e", "d"]
	}]
}`

const SUnionStore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "SADD key1 a",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 b",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key1 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 c",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 d",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SADD key2 e",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "SUNIONSTORE key key1 key2",
		"response": 5
	}, {
		"protocol": "redis",
		"request": "SMEMBERS key",
		"response": ["a", "b", "c", "e", "d"]
	}]
}`
