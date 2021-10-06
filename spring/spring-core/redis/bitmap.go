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
	CommandGetBit = "getbit"
	CommandSetBit = "setbit"
)

type BitmapCommand interface {
	GetBit(ctx context.Context, key string, offset int64) (int64, error)
	SetBit(ctx context.Context, key string, offset int64, value int) (int64, error)
}

func (c *BaseClient) GetBit(ctx context.Context, key string, offset int64) (int64, error) {
	args := []interface{}{CommandGetBit, key, offset}
	return c.Int64(ctx, args...)
}

func (c *BaseClient) SetBit(ctx context.Context, key string, offset int64, value int) (int64, error) {
	args := []interface{}{CommandSetBit, key, offset, value}
	return c.Int64(ctx, args...)
}
