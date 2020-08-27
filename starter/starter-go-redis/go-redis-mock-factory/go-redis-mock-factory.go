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

package GoRedisMockFactory

import (
	"github.com/elliotchance/redismock"
	"github.com/go-redis/redis"
)

// MockRedisClient 创建 Redis Mock 客户端
func MockRedisClient(fn func(*redismock.ClientMock)) func() redis.Cmdable {
	return func() redis.Cmdable {
		mock := redismock.NewMock()
		fn(mock)
		return mock
	}
}
