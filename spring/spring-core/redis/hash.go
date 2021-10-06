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

package redis

import (
	"context"
)

const (
	CommandHGet = "hget"
	CommandHSet = "hset"
)

type HashCommand interface {
	HGet(ctx context.Context, key string, field string) (string, error)
	HSet(ctx context.Context, key string, values ...interface{}) (int64, error)
}

func (c *BaseClient) HGet(ctx context.Context, key string, field string) (string, error) {
	reply, err := c.Do(ctx, CommandHGet, key, field)
	if err != nil {
		return "", err
	}
	return reply.String(), nil
}

func (c *BaseClient) HSet(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandHSet, key}
	args = append(args, values...)
	reply, err := c.Do(ctx, args...)
	if err != nil {
		return -1, err
	}
	return reply.Int64(), nil
}
