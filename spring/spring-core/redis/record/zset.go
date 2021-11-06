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

func ZAdd(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 1 uno","response":1},{"protocol":"redis","request":"ZADD myzset 2 two 3 three","response":2},{"protocol":"redis","request":"ZRANGE myzset 0 -1 WITHSCORES","response":[["one","1"],["uno","1"],["two","2"],["three","3"]]}]}`
	RunCase(t, c, testcases.ZAdd, str)
}

func ZCard(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZCARD myzset","response":2}]}`
	RunCase(t, c, testcases.ZCard, str)
}

func ZCount(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZCOUNT myzset -inf +inf","response":3},{"protocol":"redis","request":"ZCOUNT myzset (1 3","response":2}]}`
	RunCase(t, c, testcases.ZCount, str)
}

func ZDiff(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD zset1 1 one","response":1},{"protocol":"redis","request":"ZADD zset1 2 two","response":1},{"protocol":"redis","request":"ZADD zset1 3 three","response":1},{"protocol":"redis","request":"ZADD zset2 1 one","response":1},{"protocol":"redis","request":"ZADD zset2 2 two","response":1},{"protocol":"redis","request":"ZDIFF 2 zset1 zset2","response":["three"]},{"protocol":"redis","request":"ZDIFF 2 zset1 zset2 WITHSCORES","response":[["three","3"]]}]}`
	RunCase(t, c, testcases.ZDiff, str)
}

func ZIncrBy(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZINCRBY myzset 2 one","response":"3"},{"protocol":"redis","request":"ZRANGE myzset 0 -1 WITHSCORES","response":[["two","2"],["one","3"]]}]}`
	RunCase(t, c, testcases.ZIncrBy, str)
}

func ZInter(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD zset1 1 one","response":1},{"protocol":"redis","request":"ZADD zset1 2 two","response":1},{"protocol":"redis","request":"ZADD zset2 1 one","response":1},{"protocol":"redis","request":"ZADD zset2 2 two","response":1},{"protocol":"redis","request":"ZADD zset2 3 three","response":1},{"protocol":"redis","request":"ZINTER 2 zset1 zset2","response":["one","two"]},{"protocol":"redis","request":"ZINTER 2 zset1 zset2 WITHSCORES","response":[["one","2"],["two","4"]]}]}`
	RunCase(t, c, testcases.ZInter, str)
}

func ZLexCount(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 0 a 0 b 0 c 0 d 0 e","response":5},{"protocol":"redis","request":"ZADD myzset 0 f 0 g","response":2},{"protocol":"redis","request":"ZLEXCOUNT myzset - +","response":7},{"protocol":"redis","request":"ZLEXCOUNT myzset [b [f","response":5}]}`
	RunCase(t, c, testcases.ZLexCount, str)
}

func ZMScore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZMSCORE myzset one two nofield","response":["1","2",null]}]}`
	RunCase(t, c, testcases.ZMScore, str)
}

func ZPopMax(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZPOPMAX myzset","response":[["three","3"]]}]}`
	RunCase(t, c, testcases.ZPopMax, str)
}

func ZPopMin(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZPOPMIN myzset","response":[["one","1"]]}]}`
	RunCase(t, c, testcases.ZPopMin, str)
}

func ZRandMember(t *testing.T, c redis.Client) {
	str := `skip`
	RunCase(t, c, testcases.ZRandMember, str)
}

func ZRange(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZRANGE myzset 0 -1","response":["one","two","three"]},{"protocol":"redis","request":"ZRANGE myzset 2 3","response":["three"]},{"protocol":"redis","request":"ZRANGE myzset -2 -1","response":["two","three"]},{"protocol":"redis","request":"ZRANGE myzset 0 1 WITHSCORES","response":[["one","1"],["two","2"]]}]}`
	RunCase(t, c, testcases.ZRange, str)
}

func ZRangeByLex(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g","response":7},{"protocol":"redis","request":"ZRANGEBYLEX myzset - [c","response":["a","b","c"]},{"protocol":"redis","request":"ZRANGEBYLEX myzset - (c","response":["a","b"]},{"protocol":"redis","request":"ZRANGEBYLEX myzset [aaa (g","response":["b","c","d","e","f"]}]}`
	RunCase(t, c, testcases.ZRangeByLex, str)
}

func ZRangeByScore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZRANGEBYSCORE myzset -inf +inf","response":["one","two","three"]},{"protocol":"redis","request":"ZRANGEBYSCORE myzset 1 2","response":["one","two"]},{"protocol":"redis","request":"ZRANGEBYSCORE myzset (1 2","response":["two"]},{"protocol":"redis","request":"ZRANGEBYSCORE myzset (1 (2","response":[]}]}`
	RunCase(t, c, testcases.ZRangeByScore, str)
}

func ZRank(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZRANK myzset three","response":2}]}`
	RunCase(t, c, testcases.ZRank, str)
}

func ZRem(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZREM myzset two","response":1},{"protocol":"redis","request":"ZRANGE myzset 0 -1 WITHSCORES","response":[["one","1"],["three","3"]]}]}`
	RunCase(t, c, testcases.ZRem, str)
}

func ZRemRangeByLex(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 0 aaaa 0 b 0 c 0 d 0 e","response":5},{"protocol":"redis","request":"ZADD myzset 0 foo 0 zap 0 zip 0 ALPHA 0 alpha","response":5},{"protocol":"redis","request":"ZRANGE myzset 0 -1","response":["ALPHA","aaaa","alpha","b","c","d","e","foo","zap","zip"]},{"protocol":"redis","request":"ZREMRANGEBYLEX myzset [alpha [omega","response":6},{"protocol":"redis","request":"ZRANGE myzset 0 -1","response":["ALPHA","aaaa","zap","zip"]}]}`
	RunCase(t, c, testcases.ZRemRangeByLex, str)
}

func ZRemRangeByRank(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZREMRANGEBYRANK myzset 0 1","response":2},{"protocol":"redis","request":"ZRANGE myzset 0 -1 WITHSCORES","response":[["three","3"]]}]}`
	RunCase(t, c, testcases.ZRemRangeByRank, str)
}

func ZRemRangeByScore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZREMRANGEBYSCORE myzset -inf (2","response":1},{"protocol":"redis","request":"ZRANGE myzset 0 -1 WITHSCORES","response":[["two","2"],["three","3"]]}]}`
	RunCase(t, c, testcases.ZRemRangeByScore, str)
}

func ZRevRange(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZREVRANGE myzset 0 -1","response":["three","two","one"]},{"protocol":"redis","request":"ZREVRANGE myzset 2 3","response":["one"]},{"protocol":"redis","request":"ZREVRANGE myzset -2 -1","response":["two","one"]}]}`
	RunCase(t, c, testcases.ZRevRange, str)
}

func ZRevRangeByLex(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 0 a 0 b 0 c 0 d 0 e 0 f 0 g","response":7},{"protocol":"redis","request":"ZREVRANGEBYLEX myzset [c -","response":["c","b","a"]},{"protocol":"redis","request":"ZREVRANGEBYLEX myzset (c -","response":["b","a"]},{"protocol":"redis","request":"ZREVRANGEBYLEX myzset (g [aaa","response":["f","e","d","c","b"]}]}`
	RunCase(t, c, testcases.ZRevRangeByLex, str)
}

func ZRevRangeByScore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZREVRANGEBYSCORE myzset +inf -inf","response":["three","two","one"]},{"protocol":"redis","request":"ZREVRANGEBYSCORE myzset 2 1","response":["two","one"]},{"protocol":"redis","request":"ZREVRANGEBYSCORE myzset 2 (1","response":["two"]},{"protocol":"redis","request":"ZREVRANGEBYSCORE myzset (2 (1","response":[]}]}`
	RunCase(t, c, testcases.ZRevRangeByScore, str)
}

func ZRevRank(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZADD myzset 2 two","response":1},{"protocol":"redis","request":"ZADD myzset 3 three","response":1},{"protocol":"redis","request":"ZREVRANK myzset one","response":2}]}`
	RunCase(t, c, testcases.ZRevRank, str)
}

func ZScore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD myzset 1 one","response":1},{"protocol":"redis","request":"ZSCORE myzset one","response":"1"}]}`
	RunCase(t, c, testcases.ZScore, str)
}

func ZUnion(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD zset1 1 one","response":1},{"protocol":"redis","request":"ZADD zset1 2 two","response":1},{"protocol":"redis","request":"ZADD zset2 1 one","response":1},{"protocol":"redis","request":"ZADD zset2 2 two","response":1},{"protocol":"redis","request":"ZADD zset2 3 three","response":1},{"protocol":"redis","request":"ZUNION 2 zset1 zset2","response":["one","three","two"]},{"protocol":"redis","request":"ZUNION 2 zset1 zset2 WITHSCORES","response":[["one","2"],["three","3"],["two","4"]]}]}`
	RunCase(t, c, testcases.ZUnion, str)
}

func ZUnionStore(t *testing.T, c redis.Client) {
	str := `{"session":"df3b64266ebe4e63a464e135000a07cd","inbound":{},"actions":[{"protocol":"redis","request":"ZADD zset1 1 one","response":1},{"protocol":"redis","request":"ZADD zset1 2 two","response":1},{"protocol":"redis","request":"ZADD zset2 1 one","response":1},{"protocol":"redis","request":"ZADD zset2 2 two","response":1},{"protocol":"redis","request":"ZADD zset2 3 three","response":1},{"protocol":"redis","request":"ZUNIONSTORE out 2 zset1 zset2 WEIGHTS 2 3","response":3},{"protocol":"redis","request":"ZRANGE out 0 -1 WITHSCORES","response":[["one","5"],["three","9"],["two","10"]]}]}`
	RunCase(t, c, testcases.ZUnionStore, str)
}
