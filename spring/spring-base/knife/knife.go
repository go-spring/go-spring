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
	"errors"
	"fmt"
	"reflect"
	"sync"
)

type ctxKeyType int

var ctxKey ctxKeyType

var ErrUninitialized = errors.New("knife uninitialized")

func cache(ctx context.Context) (*sync.Map, bool) {
	c, ok := ctx.Value(ctxKey).(*sync.Map)
	return c, ok
}

// New 返回带有缓存空间的 context.Context 对象。
func New(ctx context.Context) context.Context {
	if _, ok := cache(ctx); ok {
		return ctx
	}
	return context.WithValue(ctx, ctxKey, new(sync.Map))
}

// Copy 拷贝 context.Context 对象中的内容到另一个 context.Context 对象。
func Copy(src context.Context) context.Context {
	if v, ok := cache(src); ok {
		return context.WithValue(context.Background(), ctxKey, v)
	}
	return context.WithValue(context.Background(), ctxKey, new(sync.Map))
}

// Get 从 context.Context 对象中获取 key 对应的 val。
func Get(ctx context.Context, key string) (interface{}, bool) {
	if c, ok := cache(ctx); ok {
		return c.Load(key)
	}
	return nil, false
}

// Fetch 从 context.Context 对象中获取 key 对应的 val。
func Fetch(ctx context.Context, key string, out interface{}) (bool, error) {

	ov := reflect.ValueOf(out)
	if ov.Kind() != reflect.Ptr || ov.IsNil() {
		return false, errors.New("out should be ptr and not nil")
	}

	v, ok := Get(ctx, key)
	if !ok {
		return false, nil
	}

	ev := ov.Elem()
	rv := reflect.ValueOf(v)
	if rv.Type() != ev.Type() {
		return false, fmt.Errorf("want %s but got %T", ev.Type(), v)
	}

	ev.Set(rv)
	return true, nil
}

// Set 将 key 及其 val 保存到 context.Context 对象。
func Set(ctx context.Context, key string, val interface{}) error {
	c, ok := cache(ctx)
	if !ok {
		return ErrUninitialized
	}
	if _, loaded := c.LoadOrStore(key, val); loaded {
		return fmt.Errorf("duplicate set %s value", key)
	}
	return nil
}
