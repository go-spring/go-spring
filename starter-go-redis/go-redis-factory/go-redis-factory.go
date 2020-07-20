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

package GoRedisFactory

import (
	"fmt"
	"github.com/go-redis/redis"
	StarterRedis "github.com/go-spring/go-spring/starter-redis"
)

// NewGoRedisClient 创建 redis 客户端
func NewGoRedisClient(config StarterRedis.RedisConfig) (redis.Cmdable, error) {

	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client := redis.NewClient(&redis.Options{
		Addr:     address,
		Password: config.Password,
		DB:       config.Database,
	})

	if err := client.Ping().Err(); err != nil {
		return nil, err
	}
	return client, nil
}
