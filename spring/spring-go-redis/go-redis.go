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
	"context"
	"fmt"

	g "github.com/go-redis/redis/v8"
	"github.com/go-spring/spring-base/fastdev"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/redis"
)

type client struct {
	redis.BaseClient
	client *g.Client
}

// NewClient 创建 Redis 客户端
func NewClient(config conf.RedisClientConfig) (redis.Client, error) {

	if fastdev.ReplayMode() {
		return &redis.BaseClient{}, nil
	}

	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	clt := g.NewClient(&g.Options{
		Addr:     address,
		Password: config.Password,
		DB:       config.Database,
	})

	if config.Ping {
		if err := clt.Ping(context.Background()).Err(); err != nil {
			return nil, err
		}
	}

	c := &client{client: clt}
	c.DoFunc = c.do
	return c, nil
}

func (c *client) do(ctx context.Context, args ...interface{}) (interface{}, error) {
	cmd := c.client.Do(ctx, args...)
	_, err := cmd.Result()
	if err != nil {
		if err == g.Nil {
			return nil, redis.ErrNil
		}
		return nil, err
	}
	return cmd.Val(), nil
}
