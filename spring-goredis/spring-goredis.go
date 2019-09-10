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

package SpringGoRedis

import (
	"github.com/go-redis/redis"
	"github.com/didichuxing/go-spring/spring-core"
	Logger "github.com/didichuxing/go-spring/spring-logger"
)

type GoRedisTemplate struct {
	Client *redis.Client
}

func (redisTemplate *GoRedisTemplate) InitBean(context SpringCore.SpringContext) (err error) {
	redisTemplate.Client = redis.NewClient(&redis.Options{
		Addr: context.GetProperties("redis.address"),
	})
	return
}

func (redisTemplate *GoRedisTemplate) HGetAll(key string) (map[string]string, error) {
	result := redisTemplate.Client.HGetAll(key)
	if result.Err() != nil {
		Logger.Errorln(result.Err())
		return nil, result.Err()
	}
	return result.Val(), nil
}

func (redisTemplate *GoRedisTemplate) Get(key string) (string, error) {
	result := redisTemplate.Client.Get(key)
	if result.Err() != nil {
		Logger.Errorln(result.Err())
		return "", result.Err()
	}
	return result.Val(), nil
}

func (redisTemplate *GoRedisTemplate) Set(key, val string) error {
	result := redisTemplate.Client.Set(key, val, 0)
	if result.Err() != nil {
		Logger.Errorln(result.Err())
		return result.Err()
	}
	return nil
}
