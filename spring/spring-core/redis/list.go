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
	CommandLPush = "lpush"
	CommandLPop  = "lpop"
	CommandRPush = "rpush"
	CommandRPop  = "rpop"
)

type ListCommand interface {
	LPush(ctx context.Context, key string, values ...interface{}) (int64, error)
	LPop(ctx context.Context, key string) (string, error)
	RPush(ctx context.Context, key string, values ...interface{}) (int64, error)
	RPop(ctx context.Context, key string) (string, error)
}

func (c *BaseClient) LPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandLPush, key}
	args = append(args, values...)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) LPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandLPop, key}
	return c.String(ctx, args...)
}

func (c *BaseClient) RPush(ctx context.Context, key string, values ...interface{}) (int64, error) {
	args := []interface{}{CommandRPush, key}
	args = append(args, values...)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) RPop(ctx context.Context, key string) (string, error) {
	args := []interface{}{CommandRPop, key}
	return c.String(ctx, args...)
}
