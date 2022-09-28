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
	"sync"
)

// errUninitialized returns this error when knife is uninitialized.
var errUninitialized = errors.New("knife uninitialized")

var ctxKey int

func cache(ctx context.Context) (*sync.Map, bool) {
	m, ok := ctx.Value(&ctxKey).(*sync.Map)
	return m, ok
}

// New returns a new context.Context with a *sync.Map bound. If a *sync.Map
// is already bound, then returns the input context.Context.
func New(ctx context.Context) (_ context.Context, cached bool) {
	if _, ok := cache(ctx); ok {
		return ctx, true
	}
	ctx = context.WithValue(ctx, &ctxKey, new(sync.Map))
	return ctx, false
}

// Load returns the value stored for a key, or nil if no value is present.
// If knife is uninitialized, the error of knife uninitialized will be returned.
func Load(ctx context.Context, key string) (interface{}, error) {
	m, ok := cache(ctx)
	if !ok {
		return nil, errUninitialized
	}
	v, _ := m.Load(key)
	return v, nil
}

// Store stores the key and value in the context.Context, if the key is already
// in the context.Context, the error of duplicate key will be returned.
// If knife is uninitialized, the error of knife uninitialized will be returned.
func Store(ctx context.Context, key string, val interface{}) error {
	m, ok := cache(ctx)
	if !ok {
		return errUninitialized
	}
	if _, loaded := m.LoadOrStore(key, val); loaded {
		return fmt.Errorf("duplicate key %s", key)
	}
	return nil
}

// LoadOrStore returns the existing value for the key if present. Otherwise, it
// stores and returns the given value. The loaded result is true if the value was
// loaded, false if stored.
// If knife is uninitialized, the error of knife uninitialized will be returned.
func LoadOrStore(ctx context.Context, key string, val interface{}) (actual interface{}, loaded bool, err error) {
	m, ok := cache(ctx)
	if !ok {
		return nil, false, errUninitialized
	}
	actual, loaded = m.LoadOrStore(key, val)
	return
}

// Delete deletes the key and value.
func Delete(ctx context.Context, key string) {
	if m, ok := cache(ctx); ok {
		m.Delete(key)
	}
}

// Range calls f sequentially for each key and value.
func Range(ctx context.Context, f func(key, value interface{}) bool) {
	if m, ok := cache(ctx); ok {
		m.Range(f)
	}
}
