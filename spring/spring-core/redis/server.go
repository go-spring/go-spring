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
	CommandFlushAll = "FLUSHALL"
)

type ServerCommand interface {

	// FlushAll https://redis.io/commands/flushall
	// Command: FLUSHALL [ASYNC|SYNC]
	// Simple string reply
	FlushAll(ctx context.Context, args ...interface{}) (string, error)
}

func (c *BaseClient) FlushAll(ctx context.Context, args ...interface{}) (string, error) {
	args = append([]interface{}{CommandFlushAll}, args...)
	return c.String(ctx, args...)
}
