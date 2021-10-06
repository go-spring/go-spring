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
	CommandSMembers = "smembers"
	CommandSAdd     = "sadd"
	CommandSRem     = "srem"
)

type SetCommand interface {
	SMembers(ctx context.Context, key string) ([]string, error)
	SAdd(ctx context.Context, key string, members ...interface{}) (int64, error)
	SRem(ctx context.Context, key string, members ...interface{}) (int64, error)
}

func (c *BaseClient) SMembers(ctx context.Context, key string) ([]string, error) {
	args := []interface{}{CommandSMembers, key}
	return c.StringSlice(ctx, args...)
}

func (c *BaseClient) SAdd(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{CommandSAdd, key}
	args = append(args, members...)
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SRem(ctx context.Context, key string, members ...interface{}) (int64, error) {
	args := []interface{}{CommandSRem, key}
	args = append(args, members...)
	return c.Int64(ctx, args...)
}
