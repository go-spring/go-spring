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

package cache

import (
	"context"
	"sync"
	"time"

	"github.com/go-spring/spring-base/run"
)

var Engine Driver = &engine{}

type Driver interface {
	Load(ctx context.Context, m *sync.Map, key string, loader ResultLoader, arg OptionArg) (loadType LoadType, result Result, err error)
}

// TODO using sharding to improve performance at high concurrency.
var cache = &sync.Map{}

// InvalidateAll delete all cached values, just for unit testing.
func InvalidateAll() {
	run.MustTestMode()
	cache = &sync.Map{}
}

type Option func(*OptionArg)

// ExpireAfterWrite sets the expiration time of the cache value.
func ExpireAfterWrite(v time.Duration) Option {
	return func(arg *OptionArg) {
		arg.ExpireAfterWrite = v
	}
}

// Loader gets value from background, such as Redis, MySQL, etc.
type Loader func(ctx context.Context, key string) (interface{}, error)

// Load loads value from cache, if there is no cached value, call the loader to get value, and then stores it.
func Load(ctx context.Context, key string, loader Loader, opts ...Option) (loadType LoadType, result Result, _ error) {
	arg := OptionArg{}
	for _, opt := range opts {
		opt(&arg)
	}
	// TODO detect whether loader has changed, so as to confirm whether loader method is called in multiple locations.
	return Engine.Load(ctx, cache, key, func(ctx context.Context, key string) (Result, error) {
		v, err := loader(ctx, key)
		if err != nil {
			return nil, err
		}
		return NewValueResult(v), nil
	}, arg)
}
