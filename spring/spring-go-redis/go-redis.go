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
	"time"

	g "github.com/go-redis/redis/v8"
	"github.com/go-spring/spring-core/redis"
)

func NewClient(config redis.Config) (*redis.Client, error) {
	connPool, err := Open(config)
	if err != nil {
		return nil, err
	}
	return redis.NewClient(connPool), nil
}

func Open(config redis.Config) (redis.Driver, error) {

	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	client := g.NewClient(&g.Options{
		Addr:         address,
		Username:     config.Username,
		Password:     config.Password,
		DB:           config.Database,
		DialTimeout:  time.Duration(config.ConnectTimeout) * time.Millisecond,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Millisecond,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Millisecond,
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Millisecond,
	})

	if config.Ping {
		if err := client.Ping(context.Background()).Err(); err != nil {
			return nil, err
		}
	}

	return &Driver{client: client}, nil
}

type Driver struct {
	client *g.Client
}

func (c *Driver) Exec(ctx context.Context, args []interface{}) (interface{}, error) {
	ret := c.client.Do(ctx, args...)
	_, err := ret.Result()
	if err != nil {
		if err == g.Nil {
			return nil, redis.ErrNil()
		}
		return nil, err
	}
	return ret.Val(), nil
}
