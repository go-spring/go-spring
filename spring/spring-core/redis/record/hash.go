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

func HDel(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HDel, str)
}

func HExists(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HExists, str)
}

func HGet(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HGet, str)
}

func HGetAll(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HGetAll, str)
}

func HIncrBy(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HIncrBy, str)
}

func HIncrByFloat(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HIncrByFloat, str)
}

func HKeys(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HKeys, str)
}

func HLen(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HLen, str)
}

func HMGet(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HMGet, str)
}

func HSet(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HSet, str)
}

func HSetNX(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HSetNX, str)
}

func HStrLen(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HStrLen, str)
}

func HVals(t *testing.T, c redis.Client) {
	str := `
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
	RunCase(t, c, testcases.HVals, str)
}
