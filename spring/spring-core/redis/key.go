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
	"time"
)

const (
	CommandDel      = "del"
	CommandExpire   = "expire"
	CommandExpireAt = "expireat"
)

type KeyCommand interface {
	Del(ctx context.Context, keys ...string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
	ExpireAt(ctx context.Context, key string, tm time.Time) (bool, error)
}

func (c *BaseClient) Del(ctx context.Context, keys ...string) (int64, error) {
	args := []interface{}{CommandDel}
	for _, key := range keys {
		args = append(args, key)
	}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) Expire(ctx context.Context, key string, expiration time.Duration) (bool, error) {
	args := []interface{}{CommandExpire, key, expiration}
	return c.Bool(ctx, args...)
}

func (c *BaseClient) ExpireAt(ctx context.Context, key string, expiration time.Time) (bool, error) {
	args := []interface{}{CommandExpireAt, key, expiration}
	return c.Bool(ctx, args...)
}
