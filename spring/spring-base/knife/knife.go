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

// Package knife 提供了 context.Context 上的缓存。
package knife

import (
	"context"
	"sync"
)

type ctxKeyType int

var ctxKey ctxKeyType

func cache(ctx context.Context) *sync.Map {
	c, _ := ctx.Value(ctxKey).(*sync.Map)
	return c
}

// New 返回带有缓存空间的 context.Context 对象。
func New(ctx context.Context) context.Context {
	if cache(ctx) != nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey, new(sync.Map))
}

// Get 从 context.Context 对象中获取 key 对应的 val。
func Get(ctx context.Context, key string) interface{} {
	if c := cache(ctx); c != nil {
		v, _ := c.Load(key)
		return v
	}
	return nil
}

// Set 将 key 及其 val 保存到 context.Context 对象。
func Set(ctx context.Context, key string, val interface{}) {
	if c := cache(ctx); c != nil {
		c.Store(key, val)
	}
}
