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

const ZAdd = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 1 uno",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two 3 three",
		"response": 2
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1 WITHSCORES",
		"response": [
			["one", "1"],
			["uno", "1"],
			["two", "2"],
			["three", "3"]
		]
	}]
}`

const ZCard = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZCARD myzset",
		"response": 2
	}]
}`

const ZCount = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZCOUNT myzset -inf +inf",
		"response": 3
	}, {
		"protocol": "redis",
		"request": "ZCOUNT myzset (1 3",
		"response": 2
	}]
}`

const ZDiff = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD zset1 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset1 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset1 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZDIFF 2 zset1 zset2",
		"response": ["three"]
	}, {
		"protocol": "redis",
		"request": "ZDIFF 2 zset1 zset2 WITHSCORES",
		"response": [
			["three", "3"]
		]
	}]
}`

const ZIncrBy = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZINCRBY myzset 2 one",
		"response": "3"
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1 WITHSCORES",
		"response": [
			["two", "2"],
			["one", "3"]
		]
	}]
}`

const ZInter = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD zset1 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset1 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZINTER 2 zset1 zset2",
		"response": ["one", "two"]
	}, {
		"protocol": "redis",
		"request": "ZINTER 2 zset1 zset2 WITHSCORES",
		"response": [
			["one", "2"],
			["two", "4"]
		]
	}]
}`

const ZLexCount = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e",
		"response": 5
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 0 f 0 g",
		"response": 2
	}, {
		"protocol": "redis",
		"request": "ZLEXCOUNT myzset - +",
		"response": 7
	}, {
		"protocol": "redis",
		"request": "ZLEXCOUNT myzset [b [f",
		"response": 5
	}]
}`

const ZMScore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZMSCORE myzset one two nofield",
		"response": ["1", "2", null]
	}]
}`

const ZPopMax = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZPOPMAX myzset",
		"response": [
			["three", "3"]
		]
	}]
}`

const ZPopMin = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZPOPMIN myzset",
		"response": [
			["one", "1"]
		]
	}]
}`

const ZRandMember = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD dadi 1 uno 2 due 3 tre 4 quattro 5 cinque 6 sei",
		"response": 6
	}, {
		"protocol": "redis",
		"request": "ZRANDMEMBER dadi",
		"response": "sei"
	}, {
		"protocol": "redis",
		"request": "ZRANDMEMBER dadi",
		"response": "sei"
	}, {
		"protocol": "redis",
		"request": "ZRANDMEMBER dadi -5 WITHSCORES",
		"response": [
			["uno", "1"],
			["uno", "1"],
			["cinque", "5"],
			["sei", "6"],
			["due", "2"]
		]
	}]
}`

const ZRange = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1",
		"response": ["one", "two", "three"]
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 2 3",
		"response": ["three"]
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset -2 -1",
		"response": ["two", "three"]
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 1 WITHSCORES",
		"response": [
			["one", "1"],
			["two", "2"]
		]
	}]
}`

const ZRangeByLex = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g",
		"response": 7
	}, {
		"protocol": "redis",
		"request": "ZRANGEBYLEX myzset - [c",
		"response": ["a", "b", "c"]
	}, {
		"protocol": "redis",
		"request": "ZRANGEBYLEX myzset - (c",
		"response": ["a", "b"]
	}, {
		"protocol": "redis",
		"request": "ZRANGEBYLEX myzset [aaa (g",
		"response": ["b", "c", "d", "e", "f"]
	}]
}`

const ZRangeByScore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZRANGEBYSCORE myzset -inf +inf",
		"response": ["one", "two", "three"]
	}, {
		"protocol": "redis",
		"request": "ZRANGEBYSCORE myzset 1 2",
		"response": ["one", "two"]
	}, {
		"protocol": "redis",
		"request": "ZRANGEBYSCORE myzset (1 2",
		"response": ["two"]
	}, {
		"protocol": "redis",
		"request": "ZRANGEBYSCORE myzset (1 (2",
		"response": []
	}]
}`

const ZRank = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZRANK myzset three",
		"response": 2
	}, {
		"protocol": "redis",
		"request": "ZRANK myzset four",
		"response": "(nil)"
	}]
}`

const ZRem = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZREM myzset two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1 WITHSCORES",
		"response": [
			["one", "1"],
			["three", "3"]
		]
	}]
}`

const ZRemRangeByLex = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 0 aaaa 0 b 0 c 0 d 0 e",
		"response": 5
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 0 foo 0 zap 0 zip 0 ALPHA 0 alpha",
		"response": 5
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1",
		"response": ["ALPHA", "aaaa", "alpha", "b", "c", "d", "e", "foo", "zap", "zip"]
	}, {
		"protocol": "redis",
		"request": "ZREMRANGEBYLEX myzset [alpha [omega",
		"response": 6
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1",
		"response": ["ALPHA", "aaaa", "zap", "zip"]
	}]
}`

const ZRemRangeByRank = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZREMRANGEBYRANK myzset 0 1",
		"response": 2
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1 WITHSCORES",
		"response": [
			["three", "3"]
		]
	}]
}`

const ZRemRangeByScore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZREMRANGEBYSCORE myzset -inf (2",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZRANGE myzset 0 -1 WITHSCORES",
		"response": [
			["two", "2"],
			["three", "3"]
		]
	}]
}`

const ZRevRange = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZREVRANGE myzset 0 -1",
		"response": ["three", "two", "one"]
	}, {
		"protocol": "redis",
		"request": "ZREVRANGE myzset 2 3",
		"response": ["one"]
	}, {
		"protocol": "redis",
		"request": "ZREVRANGE myzset -2 -1",
		"response": ["two", "one"]
	}]
}`

const ZRevRangeByLex = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g",
		"response": 7
	}, {
		"protocol": "redis",
		"request": "ZREVRANGEBYLEX myzset [c -",
		"response": ["c", "b", "a"]
	}, {
		"protocol": "redis",
		"request": "ZREVRANGEBYLEX myzset (c -",
		"response": ["b", "a"]
	}, {
		"protocol": "redis",
		"request": "ZREVRANGEBYLEX myzset (g [aaa",
		"response": ["f", "e", "d", "c", "b"]
	}]
}`

const ZRevRangeByScore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZREVRANGEBYSCORE myzset +inf -inf",
		"response": ["three", "two", "one"]
	}, {
		"protocol": "redis",
		"request": "ZREVRANGEBYSCORE myzset 2 1",
		"response": ["two", "one"]
	}, {
		"protocol": "redis",
		"request": "ZREVRANGEBYSCORE myzset 2 (1",
		"response": ["two"]
	}, {
		"protocol": "redis",
		"request": "ZREVRANGEBYSCORE myzset (2 (1",
		"response": []
	}]
}`

const ZRevRank = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD myzset 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZREVRANK myzset one",
		"response": 2
	}, {
		"protocol": "redis",
		"request": "ZREVRANK myzset four",
		"response": "(nil)"
	}]
}`

const ZScore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD myzset 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZSCORE myzset one",
		"response": "1"
	}]
}`

const ZUnion = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD zset1 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset1 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZUNION 2 zset1 zset2",
		"response": ["one", "three", "two"]
	}, {
		"protocol": "redis",
		"request": "ZUNION 2 zset1 zset2 WITHSCORES",
		"response": [
			["one", "2"],
			["three", "3"],
			["two", "4"]
		]
	}]
}`

const ZUnionStore = `
{
	"session": "df3b64266ebe4e63a464e135000a07cd",
	"inbound": {},
	"actions": [{
		"protocol": "redis",
		"request": "ZADD zset1 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset1 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 1 one",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 2 two",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZADD zset2 3 three",
		"response": 1
	}, {
		"protocol": "redis",
		"request": "ZUNIONSTORE out 2 zset1 zset2 WEIGHTS 2 3",
		"response": 3
	}, {
		"protocol": "redis",
		"request": "ZRANGE out 0 -1 WITHSCORES",
		"response": [
			["one", "5"],
			["three", "9"],
			["two", "10"]
		]
	}]
}`
