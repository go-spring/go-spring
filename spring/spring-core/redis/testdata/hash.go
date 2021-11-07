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

const HDel = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 foo",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HDEL myhash field1",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HDEL myhash field2",
		"response": 0
	}]
}`

const HExists = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 foo",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HEXISTS myhash field1",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HEXISTS myhash field2",
		"response": 0
	}]
}`

const HGet = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 foo",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HGET myhash field1",
		"response": "foo"
	}, {
		"protocol": "redis",
		"request": "HGET myhash field2",
		"response": "(nil)"
	}]
}`

const HGetAll = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HSET myhash field2 World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HGETALL myhash",
		"response": {
			"field1": "Hello",
			"field2": "World"
		}
	}]
}`

const HIncrBy = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field 5",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HINCRBY myhash field 1",
		"response": 6
	}, {
		"protocol": "redis",
		"request": "HINCRBY myhash field -1",
		"response": 5
	}, {
		"protocol": "redis",
		"request": "HINCRBY myhash field -10",
		"response": -5
	}]
}`

const HIncrByFloat = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET mykey field 10.5",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HINCRBYFLOAT mykey field 0.1",
		"response": "10.6"
	}, {
		"protocol": "redis",
		"request": "HINCRBYFLOAT mykey field -5",
		"response": "5.6"
	}, {
		"protocol": "redis",
		"request": "HSET mykey field 5000",
		"response": 0
	}, {
		"protocol": "redis",
		"request": "HINCRBYFLOAT mykey field 200",
		"response": "5200"
	}]
}`

const HKeys = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HSET myhash field2 World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HKEYS myhash",
		"response": ["field1", "field2"]
	}]
}`

const HLen = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HSET myhash field2 World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HLEN myhash",
		"response": 2
	}]
}`

const HMGet = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HSET myhash field2 World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HMGET myhash field1 field2 nofield",
		"response": ["Hello", "World", null]
	}]
}`

const HSet = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HGET myhash field1",
		"response": "Hello"
	}]
}`

const HSetNX = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSETNX myhash field Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HSETNX myhash field World",
		"response": 0
	}, {
		"protocol": "redis",
		"request": "HGET myhash field",
		"response": "Hello"
	}]
}`

const HStrLen = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash f1 HelloWorld f2 99 f3 -256",
		"response": 3
	}, {
		"protocol": "redis",
		"request": "HSTRLEN myhash f1",
		"response": 10
	}, {
		"protocol": "redis",
		"request": "HSTRLEN myhash f2",
		"response": 2
	}, {
		"protocol": "redis",
		"request": "HSTRLEN myhash f3",
		"response": 4
	}]
}`

const HVals = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "HSET myhash field1 Hello",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HSET myhash field2 World",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "HVALS myhash",
		"response": ["Hello", "World"]
	}]
}`
